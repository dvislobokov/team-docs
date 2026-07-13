package auth

import (
	"net/http"
	"strings"

	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"
)

// Middleware проверяет JWT и кладёт identity в контекст. При выключенной
// авторизации пропускает запрос, подставляя dev-пользователя. Если авторизация
// включена, но токена нет и разрешено публичное чтение (PublicRead) — пропускает
// как анонима (identity не устанавливается; запись отсечёт RequireEditor).
func Middleware(a *Authenticator, log *srog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !a.Enabled() {
				setUser(c, a.DevUser())
				return next(c)
			}

			raw := extractToken(c, a.cfg.Header)
			if raw == "" {
				if a.PublicRead() {
					return next(c) // аноним — читать можно, писать нельзя
				}
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

// RequireEditor — гард на запись: методы, изменяющие данные (POST/PUT/PATCH/
// DELETE), доступны только пользователю с правом редактирования; чтение
// (GET/HEAD/OPTIONS) пропускается. При выключенной авторизации — no-op.
// Ставится на группу /api ПОСЛЕ Middleware.
func RequireEditor(a *Authenticator, log *srog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !a.Enabled() {
				return next(c)
			}
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			return enforceEditor(c, a, next)
		}
	}
}

// RequireEditorStrict требует право редактирования независимо от HTTP-метода —
// для чувствительных операций (напр. полный экспорт БД, MCP). No-op без авторизации.
func RequireEditorStrict(a *Authenticator, log *srog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !a.Enabled() {
				return next(c)
			}
			return enforceEditor(c, a, next)
		}
	}
}

// enforceEditor: 401 если аноним, 403 если аутентифицирован, но без права правки.
func enforceEditor(c echo.Context, a *Authenticator, next echo.HandlerFunc) error {
	u, ok := FromContext(c)
	if !ok || u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if !a.CanEdit(u) {
		return echo.NewHTTPError(http.StatusForbidden, "editing not allowed for this user")
	}
	return next(c)
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
