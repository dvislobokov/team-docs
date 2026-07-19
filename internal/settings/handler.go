package settings

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
)

// Handler — /api/admin/settings (группа за RequireAdmin).
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/settings", h.list)
	g.PUT("/settings", h.set)
}

func (h *Handler) list(c echo.Context) error {
	return c.JSON(http.StatusOK, h.svc.List())
}

func (h *Handler) set(c echo.Context) error {
	var req struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if err := h.svc.Set(c.Request().Context(), req.Key, req.Value, auth.UserID(c)); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
