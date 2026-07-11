package auth

import (
	"net/http"
	"strings"

	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"
)

// Middleware проверяет JWT и кладёт identity в контекст. При выключенной
// авторизации пропускает запрос, подставляя dev-пользователя.
func Middleware(a *Authenticator, log *srog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !a.Enabled() {
				setUser(c, a.DevUser())
				return next(c)
			}

			raw := extractToken(c, a.cfg.Header)
			if raw == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization token")
			}
			u, err := a.Verify(raw)
			if err != nil {
				log.Error(err, "auth: token verification failed")
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization token")
			}
			setUser(c, u)
			return next(c)
		}
	}
}

func extractToken(c echo.Context, header string) string {
	v := c.Request().Header.Get(header)
	if v == "" {
		return ""
	}
	if len(v) >= 7 && strings.EqualFold(v[:7], "Bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return strings.TrimSpace(v)
}
