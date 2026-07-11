package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler отдаёт текущего пользователя фронтенду.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

// Register вешает GET /api/me на защищённую группу.
func (h *Handler) Register(api *echo.Group) {
	api.GET("/me", h.me)
}

func (h *Handler) me(c echo.Context) error {
	u, ok := FromContext(c)
	if !ok || u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthenticated")
	}
	return c.JSON(http.StatusOK, echo.Map{
		"authenticated": true,
		"username":      u.Username,
		"name":          u.Name,
		"email":         u.Email,
		"groups":        u.Groups,
	})
}
