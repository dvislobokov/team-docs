package pages

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/blocknote"
	"team-docs/internal/projects"
	"team-docs/internal/store"
)

// revisionThrottle — минимальный интервал между снапшотами версий одной страницы.
const revisionThrottle = 2 * time.Minute

// Handler обслуживает /api/pages/* и /api/search.
type Handler struct {
	q    *store.Queries
	pool *pgxpool.Pool // для многошаговых операций в транзакции (move)
	a    *auth.Authenticator
	log  *srog.Logger
}

func NewHandler(pool *pgxpool.Pool, a *auth.Authenticator, log *srog.Logger) *Handler {
	return &Handler{q: store.New(pool), pool: pool, a: a, log: log}
}

// pageRole — роль текущего пользователя в проекте страницы.
// ErrNoRows → страницы нет (404 у вызывающего).
func (h *Handler) pageRole(c echo.Context, pageID int64) (string, error) {
	projectID, err := h.q.GetPageProject(c.Request().Context(), pageID)
	if err != nil {
		return projects.RoleNone, err
	}
	u, _ := auth.FromContext(c)
	return projects.RoleForID(c.Request().Context(), h.q, h.a, u, projectID)
}

// guardRead: 404, если страницы нет или проект недоступен (не палим
// существование приватного контента).
func (h *Handler) guardRead(c echo.Context, pageID int64) error {
	role, err := h.pageRole(c, pageID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !projects.CanRead(role)) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "resolve project role")
	}
	return nil
}

// guardWrite: 404 для невидимых, 403 для читателей проекта.
func (h *Handler) guardWrite(c echo.Context, pageID int64) error {
	role, err := h.pageRole(c, pageID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !projects.CanRead(role)) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "resolve project role")
	}
	if !projects.CanWrite(role) {
		return echo.NewHTTPError(http.StatusForbidden, "editing not allowed in this project")
	}
	return nil
}

// Register регистрирует роуты на группе /api.
func (h *Handler) Register(api *echo.Group) {
	api.GET("/pages/tree", h.tree)
	api.POST("/pages", h.create)
	api.GET("/pages/:id", h.get)
	api.PUT("/pages/:id", h.update)
	api.PATCH("/pages/:id/move", h.move)
	api.DELETE("/pages/:id", h.delete)
	api.GET("/pages/:id/revisions", h.revisions)
	api.GET("/pages/:id/revisions/:revId", h.revision)
	api.GET("/pages/:id/markdown", h.exportMarkdown)
	api.GET("/search", h.search)
	api.GET("/tags", h.tags)
	api.GET("/trash", h.trash_)
	api.POST("/pages/:id/restore", h.restore)
	api.DELETE("/pages/:id/purge", h.purge)
}

// --- DTO ---

type pageResponse struct {
	ID        int64           `json:"id"`
	ParentID  *int64          `json:"parentId"`
	Title     string          `json:"title"`
	Icon      string          `json:"icon"`
	Content   json.RawMessage `json:"content"`
	Position  int32           `json:"position"`
	Version   int32           `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
	// Имя последнего редактора; null для страниц без авторства (старые, MCP).
	UpdatedByName *string `json:"updatedByName"`
	// CanEdit — вправе ли текущий пользователь редактировать страницу
	// (роль в проекте, §10); UI прячет кнопки правки для читателей.
	CanEdit bool     `json:"canEdit"`
	Tags    []string `json:"tags"`
}

type createRequest struct {
	ParentID *int64 `json:"parentId"`
	Title    string `json:"title"`
	// ProjectID — проект для корневых страниц (по умолчанию 'main');
	// у вложенных проект наследуется от родителя.
	ProjectID *int64 `json:"projectId"`
}

type updateRequest struct {
	Title   string          `json:"title"`
	Icon    string          `json:"icon"`
	Content json.RawMessage `json:"content"`
	Version int32           `json:"version"`
	// Tags: null — не трогать (клиенты без поддержки тегов), [] — очистить.
	Tags []string `json:"tags"`
}

type moveRequest struct {
	ParentID *int64 `json:"parentId"`
	Position int32  `json:"position"`
}

// --- Handlers ---

// tree отдаёт дерево проекта (?project=<id>; по умолчанию 'main').
func (h *Handler) tree(c echo.Context) error {
	ctx := c.Request().Context()
	var project store.Project
	var err error
	if raw := c.QueryParam("project"); raw != "" {
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid project")
		}
		project, err = h.q.GetProject(ctx, id)
	} else {
		project, err = h.q.GetProjectByKey(ctx, "main")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}
	if err != nil {
		return h.fail(c, err, "get project")
	}
	u, _ := auth.FromContext(c)
	role, err := projects.RoleFor(ctx, h.q, h.a, u, project)
	if err != nil {
		return h.fail(c, err, "resolve project role")
	}
	if !projects.CanRead(role) {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	rows, err := h.q.GetPageTree(ctx, project.ID)
	if err != nil {
		return h.fail(c, err, "get page tree")
	}
	type node struct {
		ID       int64  `json:"id"`
		ParentID *int64 `json:"parentId"`
		Title    string `json:"title"`
		Icon     string `json:"icon"`
		Position int32  `json:"position"`
	}
	out := make([]node, 0, len(rows))
	for _, r := range rows {
		out = append(out, node{ID: r.ID, ParentID: r.ParentID, Title: r.Title, Icon: r.Icon, Position: r.Position})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) create(c echo.Context) error {
	var req createRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Title == "" {
		req.Title = "Untitled"
	}
	ctx := c.Request().Context()

	// Проект: у вложенной страницы — от родителя (который должен существовать
	// и не лежать в корзине); у корневой — из запроса либо 'main'.
	var projectID int64
	if req.ParentID != nil {
		if _, err := h.q.GetPageMeta(ctx, *req.ParentID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return echo.NewHTTPError(http.StatusBadRequest, "parent page not found")
			}
			return h.fail(c, err, "create page: check parent")
		}
		pid, err := h.q.GetPageProject(ctx, *req.ParentID)
		if err != nil {
			return h.fail(c, err, "create page: parent project")
		}
		projectID = pid
	} else if req.ProjectID != nil {
		projectID = *req.ProjectID
	} else {
		p, err := h.q.GetProjectByKey(ctx, "main")
		if err != nil {
			return h.fail(c, err, "create page: default project")
		}
		projectID = p.ID
	}

	u, _ := auth.FromContext(c)
	role, err := projects.RoleForID(ctx, h.q, h.a, u, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusBadRequest, "project not found")
	}
	if err != nil {
		return h.fail(c, err, "create page: resolve role")
	}
	if !projects.CanWrite(role) {
		return echo.NewHTTPError(http.StatusForbidden, "editing not allowed in this project")
	}

	row, err := h.q.CreatePage(ctx, store.CreatePageParams{
		ParentID:  req.ParentID,
		Title:     req.Title,
		AuthorID:  auth.UserID(c),
		ProjectID: projectID,
	})
	if err != nil {
		return h.fail(c, err, "create page")
	}
	return c.JSON(http.StatusCreated, pageResponse{
		Tags:      []string{},
		CanEdit:   true,
		ID:        row.ID,
		ParentID:  row.ParentID,
		Title:     row.Title,
		Icon:      row.Icon,
		Content:   json.RawMessage(row.Content),
		Position:  row.Position,
		Version:   row.Version,
		UpdatedAt: row.UpdatedAt.Time,
	})
}

func (h *Handler) get(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	role, err := h.pageRole(c, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !projects.CanRead(role)) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "resolve project role")
	}
	row, err := h.q.GetPage(c.Request().Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "get page")
	}
	return c.JSON(http.StatusOK, pageResponse{
		ID:            row.ID,
		ParentID:      row.ParentID,
		Title:         row.Title,
		Icon:          row.Icon,
		Content:       json.RawMessage(row.Content),
		Position:      row.Position,
		Version:       row.Version,
		UpdatedAt:     row.UpdatedAt.Time,
		UpdatedByName: row.UpdatedByName,
		CanEdit:       projects.CanWrite(role),
		Tags:          row.Tags,
	})
}

func (h *Handler) update(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req updateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if len(req.Content) == 0 {
		req.Content = json.RawMessage("[]")
	}
	if err := h.guardWrite(c, id); err != nil {
		return err
	}

	ctx := c.Request().Context()
	row, err := h.q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          id,
		Title:       req.Title,
		Content:     []byte(req.Content),
		ContentText: ExtractText(req.Content),
		Version:     req.Version,
		Icon:        req.Icon,
		AuthorID:    auth.UserID(c),
		Tags:        normalizeTags(req.Tags),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Либо страница не существует, либо конфликт версий.
		if _, e := h.q.GetPage(ctx, id); errors.Is(e, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "page not found")
		}
		return echo.NewHTTPError(http.StatusConflict, "version conflict: page was modified")
	}
	if err != nil {
		return h.fail(c, err, "update page")
	}

	h.maybeSnapshot(ctx, row, auth.UserID(c))

	// Имя редактора в ответе — сам текущий пользователь (это он и сохранил).
	var updatedBy *string
	if u, ok := auth.FromContext(c); ok && u != nil && u.ID != 0 {
		updatedBy = &u.Name
	}
	return c.JSON(http.StatusOK, pageResponse{
		ID:            row.ID,
		ParentID:      row.ParentID,
		Title:         row.Title,
		Icon:          row.Icon,
		Content:       json.RawMessage(row.Content),
		Position:      row.Position,
		Version:       row.Version,
		UpdatedAt:     row.UpdatedAt.Time,
		UpdatedByName: updatedBy,
		CanEdit:       true, // guardWrite уже пройден
		Tags:          row.Tags,
	})
}

// maybeSnapshot записывает снапшот версии не чаще revisionThrottle на страницу.
// Ошибки только логируются — они не должны ломать основной ответ на сохранение.
func (h *Handler) maybeSnapshot(ctx context.Context, row store.UpdatePageRow, authorID *int64) {
	last, err := h.q.LatestRevisionAt(ctx, row.ID)
	if err != nil {
		h.log.Error(err, "pages: latest revision lookup failed for {ID}", row.ID)
		return
	}
	// last.Valid == false → снапшотов ещё не было, пишем первый.
	if last.Valid && time.Since(last.Time) < revisionThrottle {
		return
	}
	if err := h.q.InsertRevision(ctx, store.InsertRevisionParams{
		PageID:   row.ID,
		Version:  row.Version,
		Title:    row.Title,
		Content:  row.Content,
		AuthorID: authorID,
	}); err != nil {
		h.log.Error(err, "pages: insert revision failed for {ID}", row.ID)
	}
}

func (h *Handler) move(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req moveRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if err := h.guardWrite(c, id); err != nil {
		return err
	}
	// Перенос между проектами запрещён: родитель — из проекта страницы.
	if req.ParentID != nil {
		pageProj, err1 := h.q.GetPageProject(c.Request().Context(), id)
		parentProj, err2 := h.q.GetPageProject(c.Request().Context(), *req.ParentID)
		if err1 != nil || err2 != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "parent page not found")
		}
		if pageProj != parentProj {
			return echo.NewHTTPError(http.StatusBadRequest, "cannot move page to another project")
		}
	}
	switch err := Move(c.Request().Context(), h.pool, id, req.ParentID, req.Position); {
	case errors.Is(err, ErrPageNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	case errors.Is(err, ErrMoveIntoSubtree):
		return echo.NewHTTPError(http.StatusBadRequest, "cannot move page into its own subtree")
	case err != nil:
		return h.fail(c, err, "move page")
	}
	return c.NoContent(http.StatusNoContent)
}

// delete — мягкое удаление: страница с поддеревом уходит в корзину.
func (h *Handler) delete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardWrite(c, id); err != nil {
		return err
	}
	switch err := SoftDelete(c.Request().Context(), h.pool, id); {
	case errors.Is(err, ErrPageNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	case err != nil:
		return h.fail(c, err, "delete page")
	}
	return c.NoContent(http.StatusNoContent)
}

// normalizeTags: null остаётся null (не трогать), иначе — trim, отбрасывание
// пустых, дедупликация, максимум 20 тегов по 40 символов.
func normalizeTags(in []string) []string {
	if in == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		if len([]rune(t)) > 40 {
			t = string([]rune(t)[:40])
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 20 {
			break
		}
	}
	return out
}

// tags отдаёт теги проекта с числом страниц (?project=<id>; по умолчанию main).
func (h *Handler) tags(c echo.Context) error {
	ctx := c.Request().Context()
	var project store.Project
	var err error
	if raw := c.QueryParam("project"); raw != "" {
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid project")
		}
		project, err = h.q.GetProject(ctx, id)
	} else {
		project, err = h.q.GetProjectByKey(ctx, "main")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}
	if err != nil {
		return h.fail(c, err, "get project")
	}
	u, _ := auth.FromContext(c)
	role, err := projects.RoleFor(ctx, h.q, h.a, u, project)
	if err != nil {
		return h.fail(c, err, "resolve project role")
	}
	if !projects.CanRead(role) {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	rows, err := h.q.ListTags(ctx, project.ID)
	if err != nil {
		return h.fail(c, err, "list tags")
	}
	type tag struct {
		Tag   string `json:"tag"`
		Pages int64  `json:"pages"`
	}
	out := make([]tag, 0, len(rows))
	for _, r := range rows {
		out = append(out, tag{Tag: r.Tag, Pages: r.Pages})
	}
	return c.JSON(http.StatusOK, out)
}

// trash_ отдаёт содержимое корзины (корни удалённых поддеревьев) — только из
// проектов, где пользователь может писать (восстанавливать/удалять).
func (h *Handler) trash_(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.q.ListTrash(ctx)
	if err != nil {
		return h.fail(c, err, "list trash")
	}
	u, _ := auth.FromContext(c)
	writable := map[int64]bool{}
	type item struct {
		ID        int64     `json:"id"`
		Title     string    `json:"title"`
		Icon      string    `json:"icon"`
		DeletedAt time.Time `json:"deletedAt"`
	}
	out := []item{}
	for _, r := range rows {
		ok, seen := writable[r.ProjectID]
		if !seen {
			role, err := projects.RoleForID(ctx, h.q, h.a, u, r.ProjectID)
			if err != nil {
				return h.fail(c, err, "resolve project role")
			}
			ok = projects.CanWrite(role)
			writable[r.ProjectID] = ok
		}
		if ok {
			out = append(out, item{ID: r.ID, Title: r.Title, Icon: r.Icon, DeletedAt: r.DeletedAt.Time})
		}
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) restore(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardWrite(c, id); err != nil {
		return err
	}
	switch err := Restore(c.Request().Context(), h.pool, id); {
	case errors.Is(err, ErrPageNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "page not found in trash")
	case err != nil:
		return h.fail(c, err, "restore page")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) purge(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardWrite(c, id); err != nil {
		return err
	}
	n, err := h.q.PurgePage(c.Request().Context(), id)
	if err != nil {
		return h.fail(c, err, "purge page")
	}
	if n == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "page not found in trash")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) revisions(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardRead(c, id); err != nil {
		return err
	}
	rows, err := h.q.ListRevisions(c.Request().Context(), id)
	if err != nil {
		return h.fail(c, err, "list revisions")
	}
	type rev struct {
		ID         int64     `json:"id"`
		Version    int32     `json:"version"`
		Title      string    `json:"title"`
		CreatedAt  time.Time `json:"createdAt"`
		AuthorName *string   `json:"authorName"`
	}
	out := make([]rev, 0, len(rows))
	for _, r := range rows {
		out = append(out, rev{
			ID: r.ID, Version: r.Version, Title: r.Title,
			CreatedAt: r.CreatedAt.Time, AuthorName: r.AuthorName,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// revision отдаёт одну версию с контентом — для просмотра/отката на клиенте
// (откат = обычный PUT текущей версии с этим контентом).
func (h *Handler) revision(c echo.Context) error {
	revID, err := strconv.ParseInt(c.Param("revId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	row, err := h.q.GetRevision(c.Request().Context(), revID)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "revision not found")
	}
	if err != nil {
		return h.fail(c, err, "get revision")
	}
	return c.JSON(http.StatusOK, struct {
		ID        int64           `json:"id"`
		Version   int32           `json:"version"`
		Title     string          `json:"title"`
		Content   json.RawMessage `json:"content"`
		CreatedAt time.Time       `json:"createdAt"`
	}{
		ID:        row.ID,
		Version:   row.Version,
		Title:     row.Title,
		Content:   json.RawMessage(row.Content),
		CreatedAt: row.CreatedAt.Time,
	})
}

// exportMarkdown отдаёт страницу как .md (с заголовком H1).
func (h *Handler) exportMarkdown(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardRead(c, id); err != nil {
		return err
	}
	row, err := h.q.GetPage(c.Request().Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "export markdown")
	}
	body, err := blocknote.ToMarkdown(row.Content)
	if err != nil {
		return h.fail(c, err, "convert markdown")
	}
	md := "# " + row.Title + "\n\n" + body

	safe := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, row.Title)
	// RFC 5987: имя файла может быть кириллическим — кодируем в filename*.
	c.Response().Header().Set("Content-Disposition",
		"attachment; filename=\"page.md\"; filename*=UTF-8''"+url.PathEscape(safe+".md"))
	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
}

func (h *Handler) search(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return c.JSON(http.StatusOK, []any{})
	}
	ctx := c.Request().Context()
	u, _ := auth.FromContext(c)
	ids, err := projects.AccessibleIDs(ctx, h.q, h.a, u)
	if err != nil {
		return h.fail(c, err, "accessible projects")
	}
	if len(ids) == 0 {
		return c.JSON(http.StatusOK, []any{})
	}
	var tagFilter *string
	if t := c.QueryParam("tag"); t != "" {
		tagFilter = &t
	}
	rows, err := h.q.SearchPages(ctx, store.SearchPagesParams{
		PlaintoTsquery: q,
		ProjectIds:     ids,
		Tag:            tagFilter,
	})
	if err != nil {
		return h.fail(c, err, "search pages")
	}
	type hit struct {
		ID      int64  `json:"id"`
		Title   string `json:"title"`
		Icon    string `json:"icon"`
		Snippet string `json:"snippet"`
	}
	out := make([]hit, 0, len(rows))
	for _, r := range rows {
		out = append(out, hit{ID: r.ID, Title: r.Title, Icon: r.Icon, Snippet: string(r.Snippet)})
	}
	return c.JSON(http.StatusOK, out)
}

// --- helpers ---

func pathID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	return id, nil
}

func (h *Handler) fail(c echo.Context, err error, op string) error {
	h.log.Error(err, "pages: {Op} failed", op)
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
