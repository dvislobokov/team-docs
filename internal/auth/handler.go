package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler отдаёт текущего пользователя фронтенду.
type Handler struct{ a *Authenticator }

func NewHandler(a *Authenticator) *Handler { return &Handler{a: a} }

// Register вешает GET /api/me на группу /api.
func (h *Handler) Register(api *echo.Group) {
	api.GET("/me", h.me)
}

func (h *Handler) me(c echo.Context) error {
	u, ok := FromContext(c)
	if !ok || u == nil {
		// Анонимный доступ при PublicRead — не ошибка: отдаём статус «не вошёл».
		return c.JSON(http.StatusOK, echo.Map{"authenticated": false, "canEdit": false})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"authenticated": true,
		"canEdit":       !h.a.Enabled() || h.a.CanEdit(u),
		"isAdmin":       h.a.IsAdmin(u),
		"authEnabled":   h.a.Enabled(),
		"role":          u.Role,
		"username":      u.Username,
		"name":          u.Name,
		"email":         u.Email,
		"groups":        u.Groups,
	})
}
