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

	"team-docs/internal/blocknote"
	"team-docs/internal/store"
)

// revisionThrottle — минимальный интервал между снапшотами версий одной страницы.
const revisionThrottle = 2 * time.Minute

// Handler обслуживает /api/pages/* и /api/search.
type Handler struct {
	q    *store.Queries
	pool *pgxpool.Pool // для многошаговых операций в транзакции (move)
	log  *srog.Logger
}

func NewHandler(pool *pgxpool.Pool, log *srog.Logger) *Handler {
	return &Handler{q: store.New(pool), pool: pool, log: log}
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
}

type createRequest struct {
	ParentID *int64 `json:"parentId"`
	Title    string `json:"title"`
}

type updateRequest struct {
	Title   string          `json:"title"`
	Icon    string          `json:"icon"`
	Content json.RawMessage `json:"content"`
	Version int32           `json:"version"`
}

type moveRequest struct {
	ParentID *int64 `json:"parentId"`
	Position int32  `json:"position"`
}

// --- Handlers ---

func (h *Handler) tree(c echo.Context) error {
	rows, err := h.q.GetPageTree(c.Request().Context())
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
	row, err := h.q.CreatePage(c.Request().Context(), store.CreatePageParams{
		ParentID: req.ParentID,
		Title:    req.Title,
	})
	if err != nil {
		return h.fail(c, err, "create page")
	}
	return c.JSON(http.StatusCreated, pageResponse{
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
	row, err := h.q.GetPage(c.Request().Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}
	if err != nil {
		return h.fail(c, err, "get page")
	}
	return c.JSON(http.StatusOK, pageResponse{
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

	ctx := c.Request().Context()
	row, err := h.q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          id,
		Title:       req.Title,
		Content:     []byte(req.Content),
		ContentText: ExtractText(req.Content),
		Version:     req.Version,
		Icon:        req.Icon,
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

	h.maybeSnapshot(ctx, row)

	return c.JSON(http.StatusOK, pageResponse{
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

// maybeSnapshot записывает снапшот версии не чаще revisionThrottle на страницу.
// Ошибки только логируются — они не должны ломать основной ответ на сохранение.
func (h *Handler) maybeSnapshot(ctx context.Context, row store.UpdatePageRow) {
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
		PageID:  row.ID,
		Version: row.Version,
		Title:   row.Title,
		Content: row.Content,
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

func (h *Handler) delete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.q.DeletePage(c.Request().Context(), id); err != nil {
		return h.fail(c, err, "delete page")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) revisions(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	rows, err := h.q.ListRevisions(c.Request().Context(), id)
	if err != nil {
		return h.fail(c, err, "list revisions")
	}
	type rev struct {
		ID        int64     `json:"id"`
		Version   int32     `json:"version"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := make([]rev, 0, len(rows))
	for _, r := range rows {
		out = append(out, rev{ID: r.ID, Version: r.Version, Title: r.Title, CreatedAt: r.CreatedAt.Time})
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
	rows, err := h.q.SearchPages(c.Request().Context(), q)
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
