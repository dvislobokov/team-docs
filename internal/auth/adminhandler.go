package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"team-docs/internal/store"
)

// AdminHandler — /api/admin/*: управление пользователями и ролями (ROADMAP §9).
// Группа должна быть за RequireAdmin.
type AdminHandler struct {
	q   *store.Queries
	reg *Registry
}

func NewAdminHandler(pool *pgxpool.Pool, reg *Registry) *AdminHandler {
	return &AdminHandler{q: store.New(pool), reg: reg}
}

func (h *AdminHandler) Register(g *echo.Group) {
	g.GET("/users", h.listUsers)
	g.PUT("/users/:id/role", h.setRole)
	g.GET("/groups", h.listGroups)
	g.POST("/groups", h.createGroup)
	g.DELETE("/groups/:id", h.deleteGroup)
	g.GET("/groups/:id/members", h.groupMembers)
	g.PUT("/groups/:id/members/:userId", h.addGroupMember)
	g.DELETE("/groups/:id/members/:userId", h.removeGroupMember)
}

func (h *AdminHandler) listGroups(c echo.Context) error {
	rows, err := h.q.ListGroups(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	type group struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Members int64  `json:"members"`
	}
	out := make([]group, 0, len(rows))
	for _, r := range rows {
		out = append(out, group{ID: r.ID, Name: r.Name, Members: r.MemberCount})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *AdminHandler) createGroup(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}
	g, err := h.q.CreateGroup(c.Request().Context(), req.Name)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "группа уже существует?")
	}
	return c.JSON(http.StatusCreated, echo.Map{"id": g.ID, "name": g.Name})
}

func (h *AdminHandler) deleteGroup(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	n, err := h.q.DeleteGroup(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if n == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AdminHandler) groupMembers(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	rows, err := h.q.ListGroupMembers(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	type member struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	out := make([]member, 0, len(rows))
	for _, r := range rows {
		out = append(out, member{ID: r.ID, Name: r.Name, Username: r.Username, Email: r.Email})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *AdminHandler) addGroupMember(c echo.Context) error {
	gid, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, err2 := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err1 != nil || err2 != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.q.AddGroupMember(c.Request().Context(), store.AddGroupMemberParams{
		GroupID: gid, UserID: uid,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "группа или пользователь не найдены")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AdminHandler) removeGroupMember(c echo.Context) error {
	gid, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, err2 := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err1 != nil || err2 != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.q.RemoveGroupMember(c.Request().Context(), store.RemoveGroupMemberParams{
		GroupID: gid, UserID: uid,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AdminHandler) listUsers(c echo.Context) error {
	rows, err := h.q.ListUsers(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	type user struct {
		ID         int64     `json:"id"`
		Subject    string    `json:"subject"`
		Username   string    `json:"username"`
		Name       string    `json:"name"`
		Email      string    `json:"email"`
		Role       string    `json:"role"`
		LastSeenAt time.Time `json:"lastSeenAt"`
	}
	out := make([]user, 0, len(rows))
	for _, r := range rows {
		out = append(out, user{
			ID: r.ID, Subject: r.Subject, Username: r.Username, Name: r.Name,
			Email: r.Email, Role: r.Role, LastSeenAt: r.LastSeenAt.Time,
		})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *AdminHandler) setRole(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Role != RoleReader && req.Role != RoleEditor && req.Role != RoleAdmin {
		return echo.NewHTTPError(http.StatusBadRequest, "role must be reader|editor|admin")
	}
	if err := h.q.SetUserRole(c.Request().Context(), store.SetUserRoleParams{ID: id, Role: req.Role}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	// Роль кэшируется в Registry до 5 минут — сбрасываем, чтобы применилось сразу.
	h.reg.Reset()
	return c.NoContent(http.StatusNoContent)
}
