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
