package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// sessionCookie — имя cookie со встроенной сессией (подписанный HS256 JWT).
const sessionCookie = "td_session"

// sessionSecret возвращает секрет подписи сессий; если не задан в конфиге —
// генерирует случайный (сессии не переживут рестарт — для прода задать явно).
func (a *Authenticator) sessionSecret() []byte {
	if a.cfg.SessionSecret != "" {
		return []byte(a.cfg.SessionSecret)
	}
	a.randOnce.Do(func() {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		a.randSecret = []byte(hex.EncodeToString(b))
	})
	return a.randSecret
}

func (a *Authenticator) sessionTTL() time.Duration {
	h := a.cfg.SessionTTLHours
	if h <= 0 {
		h = 720
	}
	return time.Duration(h) * time.Hour
}

// IssueSession ставит cookie-сессию для пользователя (после OAuth-логина).
func (a *Authenticator) IssueSession(c echo.Context, u *User) error {
	claims := jwt.MapClaims{
		"sub":   u.Subject,
		"name":  u.Name,
		"pu":    u.Username,
		"email": u.Email,
		"exp":   time.Now().Add(a.sessionTTL()).Unix(),
		"iat":   time.Now().Unix(),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.sessionSecret())
	if err != nil {
		return err
	}
	c.SetCookie(&http.Cookie{
		Name:     sessionCookie,
		Value:    s,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Scheme() == "https",
		MaxAge:   int(a.sessionTTL().Seconds()),
	})
	return nil
}

// ClearSession снимает cookie-сессию (logout).
func (a *Authenticator) ClearSession(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}

// sessionUser достаёт пользователя из cookie-сессии; ErrNoSession — cookie нет.
var errNoSession = errors.New("no session")

func (a *Authenticator) sessionUser(c echo.Context) (*User, error) {
	ck, err := c.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return nil, errNoSession
	}
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(ck.Value, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected session signing method")
		}
		return a.sessionSecret(), nil
	}, jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid session")
	}
	str := func(k string) string { v, _ := claims[k].(string); return v }
	return &User{
		Subject:  str("sub"),
		Username: str("pu"),
		Name:     str("name"),
		Email:    str("email"),
	}, nil
}
