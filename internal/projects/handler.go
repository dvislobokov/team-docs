package projects

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/store"
)

var keyRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Handler — /api/projects/*.
type Handler struct {
	q   *store.Queries
	a   *auth.Authenticator
	log *srog.Logger
}

func NewHandler(pool *pgxpool.Pool, a *auth.Authenticator, log *srog.Logger) *Handler {
	return &Handler{q: store.New(pool), a: a, log: log}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/projects", h.list)
	api.POST("/projects", h.create)
	api.PUT("/projects/:id", h.update)
	api.DELETE("/projects/:id", h.delete)
	api.GET("/projects/:id/members", h.members)
	api.PUT("/projects/:id/members/:userId", h.setMember)
	api.DELETE("/projects/:id/members/:userId", h.removeMember)
	api.GET("/projects/:id/groups", h.groups)
	api.PUT("/projects/:id/groups/:groupId", h.setGroup)
	api.DELETE("/projects/:id/groups/:groupId", h.removeGroup)
}

func (h *Handler) groups(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	rows, err := h.q.ListProjectGroups(c.Request().Context(), p.ID)
	if err != nil {
		return h.fail(c, err, "list project groups")
	}
	type group struct {
		GroupID int64  `json:"groupId"`
		Role    string `json:"role"`
		Name    string `json:"name"`
	}
	out := make([]group, 0, len(rows))
	for _, r := range rows {
		out = append(out, group{GroupID: r.GroupID, Role: r.Role, Name: r.Name})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) setGroup(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid groupId")
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Role != auth.RoleReader && req.Role != auth.RoleEditor && req.Role != auth.RoleAdmin {
		return echo.NewHTTPError(http.StatusBadRequest, "role must be reader|editor|admin")
	}
	if err := h.q.UpsertProjectGroup(c.Request().Context(), store.UpsertProjectGroupParams{
		ProjectID: p.ID, GroupID: groupID, Role: req.Role,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "группа не найдена")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) removeGroup(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid groupId")
	}
	if err := h.q.RemoveProjectGroup(c.Request().Context(), store.RemoveProjectGroupParams{
		ProjectID: p.ID, GroupID: groupID,
	}); err != nil {
		return h.fail(c, err, "remove project group")
	}
	return c.NoContent(http.StatusNoContent)
}

type projectDTO struct {
	ID         int64     `json:"id"`
	Key        string    `json:"key"`
	Name       string    `json:"name"`
	Icon       string    `json:"icon"`
	Visibility string    `json:"visibility"`
	MyRole     string    `json:"myRole"`
	CreatedAt  time.Time `json:"createdAt"`
}

func dto(p store.Project, role string) projectDTO {
	return projectDTO{ID: p.ID, Key: p.Key, Name: p.Name, Icon: p.Icon,
		Visibility: p.Visibility, MyRole: role, CreatedAt: p.CreatedAt.Time}
}

// list — проекты, видимые текущему пользователю, с его ролью.
func (h *Handler) list(c echo.Context) error {
	ctx := c.Request().Context()
	u, _ := auth.FromContext(c)
	all, err := h.q.ListProjects(ctx)
	if err != nil {
		return h.fail(c, err, "list projects")
	}
	out := []projectDTO{}
	for _, p := range all {
		role, err := RoleFor(ctx, h.q, h.a, u, p)
		if err != nil {
			return h.fail(c, err, "resolve role")
		}
		if CanRead(role) {
			out = append(out, dto(p, role))
		}
	}
	return c.JSON(http.StatusOK, out)
}

// requireProjectAdmin возвращает проект, если текущий пользователь —
// админ проекта (или глобальный админ).
func (h *Handler) requireProjectAdmin(c echo.Context) (store.Project, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return store.Project{}, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	p, err := h.q.GetProject(c.Request().Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Project{}, echo.NewHTTPError(http.StatusNotFound, "project not found")
	}
	if err != nil {
		return store.Project{}, h.fail(c, err, "get project")
	}
	u, _ := auth.FromContext(c)
	role, err := RoleFor(c.Request().Context(), h.q, h.a, u, p)
	if err != nil {
		return store.Project{}, h.fail(c, err, "resolve role")
	}
	if !IsProjectAdmin(role) {
		return store.Project{}, echo.NewHTTPError(http.StatusForbidden, "project admin role required")
	}
	return p, nil
}

type projectBody struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	Visibility string `json:"visibility"`
}

func validVisibility(v string) bool {
	return v == VisPublic || v == VisInternal || v == VisPrivate
}

// create — только глобальный админ.
func (h *Handler) create(c echo.Context) error {
	u, _ := auth.FromContext(c)
	if !h.a.IsAdmin(u) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}
	var req projectBody
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if !keyRe.MatchString(req.Key) {
		return echo.NewHTTPError(http.StatusBadRequest, "key: латиница/цифры/дефис, до 32 символов")
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}
	if !validVisibility(req.Visibility) {
		req.Visibility = VisInternal
	}
	p, err := h.q.CreateProject(c.Request().Context(), store.CreateProjectParams{
		Key: req.Key, Name: req.Name, Icon: req.Icon, Visibility: req.Visibility,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "не удалось создать проект (ключ занят?)")
	}
	return c.JSON(http.StatusCreated, dto(p, auth.RoleAdmin))
}

func (h *Handler) update(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	var req projectBody
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Name == "" {
		req.Name = p.Name
	}
	if !validVisibility(req.Visibility) {
		req.Visibility = p.Visibility
	}
	upd, err := h.q.UpdateProject(c.Request().Context(), store.UpdateProjectParams{
		ID: p.ID, Name: req.Name, Icon: req.Icon, Visibility: req.Visibility,
	})
	if err != nil {
		return h.fail(c, err, "update project")
	}
	return c.JSON(http.StatusOK, dto(upd, auth.RoleAdmin))
}

// delete — глобальный админ; только пустой проект и не 'main'.
func (h *Handler) delete(c echo.Context) error {
	u, _ := auth.FromContext(c)
	if !h.a.IsAdmin(u) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if n, err := h.q.CountProjectPages(c.Request().Context(), id); err != nil {
		return h.fail(c, err, "count pages")
	} else if n > 0 {
		return echo.NewHTTPError(http.StatusConflict, "проект не пуст — сначала перенесите или удалите страницы")
	}
	n, err := h.q.DeleteProject(c.Request().Context(), id)
	if err != nil {
		return h.fail(c, err, "delete project")
	}
	if n == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "проект не найден или это 'main'")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) members(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	rows, err := h.q.ListProjectMembers(c.Request().Context(), p.ID)
	if err != nil {
		return h.fail(c, err, "list members")
	}
	type member struct {
		UserID   int64  `json:"userId"`
		Role     string `json:"role"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	out := make([]member, 0, len(rows))
	for _, r := range rows {
		out = append(out, member{UserID: r.UserID, Role: r.Role, Name: r.Name, Username: r.Username, Email: r.Email})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) setMember(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid userId")
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Role != auth.RoleReader && req.Role != auth.RoleEditor && req.Role != auth.RoleAdmin {
		return echo.NewHTTPError(http.StatusBadRequest, "role must be reader|editor|admin")
	}
	if err := h.q.UpsertProjectMember(c.Request().Context(), store.UpsertProjectMemberParams{
		ProjectID: p.ID, UserID: userID, Role: req.Role,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "пользователь не найден")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) removeMember(c echo.Context) error {
	p, err := h.requireProjectAdmin(c)
	if err != nil {
		return err
	}
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid userId")
	}
	if err := h.q.RemoveProjectMember(c.Request().Context(), store.RemoveProjectMemberParams{
		ProjectID: p.ID, UserID: userID,
	}); err != nil {
		return h.fail(c, err, "remove member")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) fail(c echo.Context, err error, op string) error {
	h.log.Error(err, "projects: {Op} failed", op)
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
