// Package auth валидирует JWT, выданный IAM-прокси, и кладёт identity в контекст.
// Авторизация опциональна: при Enabled=false приложение работает без неё.
package auth

import "github.com/labstack/echo/v4"

const contextKey = "auth.user"

// User — identity текущего запроса (из claims токена либо dev-пользователь).
type User struct {
	Subject  string   `json:"sub"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
}

func setUser(c echo.Context, u *User) { c.Set(contextKey, u) }

// FromContext возвращает пользователя, установленного middleware.
func FromContext(c echo.Context) (*User, bool) {
	u, ok := c.Get(contextKey).(*User)
	return u, ok
}
