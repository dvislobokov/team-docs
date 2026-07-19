package auth_test

// Юнит-тесты авторизации (HS256, без БД и без JWKS): режимы middleware,
// гарды чтение/запись, editorGroups.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/config"
)

const secret = "test-secret"

func newApp(t *testing.T, cfg config.AuthSettings) (*echo.Echo, *auth.Authenticator) {
	t.Helper()
	a, err := auth.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	log := srog.NewConsole()
	t.Cleanup(func() { _ = log.Close() })

	e := echo.New()
	api := e.Group("/api")
	api.Use(auth.Middleware(a, nil, log)) // registry nil — БД в юнит-тестах нет
	api.Use(auth.RequireEditor(a, log))
	api.GET("/probe", func(c echo.Context) error {
		u, _ := auth.FromContext(c)
		name := ""
		if u != nil {
			name = u.Name
		}
		return c.String(http.StatusOK, name)
	})
	api.POST("/probe", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	return e, a
}

func token(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func do(e *echo.Echo, method, path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestOpenModeUsesDevUser(t *testing.T) {
	e, _ := newApp(t, config.AuthSettings{Enabled: false, DevUser: "Разработчик"})
	rec := do(e, http.MethodGet, "/api/probe", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "Разработчик" {
		t.Fatalf("открытый режим: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := do(e, http.MethodPost, "/api/probe", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("запись в открытом режиме должна проходить, code=%d", rec.Code)
	}
}

func TestPublicReadAnonymous(t *testing.T) {
	cfg := config.AuthSettings{Enabled: true, HMACSecret: secret, PublicRead: true, Header: "Authorization"}
	e, _ := newApp(t, cfg)

	// Аноним читает, но не пишет.
	if rec := do(e, http.MethodGet, "/api/probe", ""); rec.Code != http.StatusOK {
		t.Fatalf("анонимное чтение при publicRead: code=%d", rec.Code)
	}
	if rec := do(e, http.MethodPost, "/api/probe", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("анонимная запись должна давать 401, code=%d", rec.Code)
	}
}

func TestClosedReadWithoutToken(t *testing.T) {
	cfg := config.AuthSettings{Enabled: true, HMACSecret: secret, PublicRead: false, Header: "Authorization"}
	e, _ := newApp(t, cfg)
	if rec := do(e, http.MethodGet, "/api/probe", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("чтение без токена при закрытом режиме: code=%d, ожидался 401", rec.Code)
	}
}

func TestValidAndInvalidToken(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: true, HMACSecret: secret, Header: "Authorization",
		NameClaim: "name", UsernameClaim: "preferred_username", EmailClaim: "email",
	}
	e, _ := newApp(t, cfg)

	ok := token(t, jwt.MapClaims{"sub": "u1", "name": "Алиса"})
	rec := do(e, http.MethodGet, "/api/probe", ok)
	if rec.Code != http.StatusOK || rec.Body.String() != "Алиса" {
		t.Fatalf("валидный токен: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := do(e, http.MethodPost, "/api/probe", ok); rec.Code != http.StatusNoContent {
		t.Fatalf("запись с валидным токеном (без editorGroups): code=%d", rec.Code)
	}

	// Чужой секрет → 401.
	bad, _ := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("other"))
	if rec := do(e, http.MethodGet, "/api/probe", bad); rec.Code != http.StatusUnauthorized {
		t.Fatalf("токен с чужой подписью: code=%d, ожидался 401", rec.Code)
	}

	// Просроченный → 401.
	expired := token(t, jwt.MapClaims{"sub": "u1", "exp": time.Now().Add(-time.Hour).Unix()})
	if rec := do(e, http.MethodGet, "/api/probe", expired); rec.Code != http.StatusUnauthorized {
		t.Fatalf("просроченный токен: code=%d, ожидался 401", rec.Code)
	}
}

func TestEditorGroups(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: true, HMACSecret: secret, Header: "Authorization",
		EditorGroups: []string{"docs-editors"},
	}
	e, _ := newApp(t, cfg)

	reader := token(t, jwt.MapClaims{"sub": "r", "groups": []string{"other"}})
	if rec := do(e, http.MethodPost, "/api/probe", reader); rec.Code != http.StatusForbidden {
		t.Fatalf("запись без группы редактора: code=%d, ожидался 403", rec.Code)
	}
	if rec := do(e, http.MethodGet, "/api/probe", reader); rec.Code != http.StatusOK {
		t.Fatalf("чтение без группы редактора: code=%d, ожидался 200", rec.Code)
	}

	editor := token(t, jwt.MapClaims{"sub": "e", "groups": []string{"docs-editors"}})
	if rec := do(e, http.MethodPost, "/api/probe", editor); rec.Code != http.StatusNoContent {
		t.Fatalf("запись с группой редактора: code=%d", rec.Code)
	}

	// Keycloak-стиль: roles в realm_access.
	kc := token(t, jwt.MapClaims{"sub": "k", "realm_access": map[string]any{"roles": []string{"docs-editors"}}})
	if rec := do(e, http.MethodPost, "/api/probe", kc); rec.Code != http.StatusNoContent {
		t.Fatalf("realm_access.roles: code=%d", rec.Code)
	}
}
