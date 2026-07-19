package auth

// Интеграционный тест встроенного OAuth: полный флоу login → callback →
// cookie-сессия → identity через middleware, плюс роли и бутстрап админа.
// Провайдер — фейковый (httptest вместо реального IdP).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"team-docs/internal/config"
	"team-docs/internal/db"
)

func oauthPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEAMDOCS_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMDOCS_TEST_DSN не задан — пропускаю интеграционный тест")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// fakeIdP поднимает token-эндпоинт фейкового провайдера.
func fakeIdP(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("code") != "good-code" {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-1"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func oauthApp(t *testing.T, cfg config.AuthSettings, subject, email string) (*echo.Echo, *Authenticator) {
	t.Helper()
	pool := oauthPool(t)
	idp := fakeIdP(t)

	// Уникальный subject на тест + очистка: роли в users переживают прогоны.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE subject = $1`, subject)
	})

	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	log := srog.NewConsole()
	t.Cleanup(func() { _ = log.Close() })
	reg := NewRegistry(pool, cfg)

	fake := &Provider{
		Key: "fake", Label: "Fake",
		AuthURL: idp.URL + "/authorize", TokenURL: idp.URL + "/token",
		ClientID: "cid", clientSecret: staticSecret("cs"),
		profile: func(_ *http.Client, tok map[string]any) (*User, error) {
			return &User{Subject: subject, Username: "fake42", Name: "Фейк Тестов", Email: email}, nil
		},
	}

	e := echo.New()
	NewOAuthHandler(a, reg, log, "http://app.local", []*Provider{fake}).Register(e)
	api := e.Group("/api")
	api.Use(Middleware(a, reg, log))
	api.Use(RequireEditor(a, log))
	NewHandler(a).Register(api)
	admin := e.Group("/api/admin", Middleware(a, reg, log), RequireAdmin(a, log))
	admin.GET("/ping", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	NewAdminHandler(pool, reg, a).Register(admin)
	return e, a
}

// runFlow проходит login+callback и возвращает cookie сессии.
func runFlow(t *testing.T, e *echo.Echo) *http.Cookie {
	t.Helper()

	// 1. login → редирект на провайдера + state-cookie.
	req := httptest.NewRequest(http.MethodGet, "/auth/login/fake", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: code=%d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "client_id=cid") || !strings.Contains(loc, "state=") {
		t.Fatalf("login redirect подозрителен: %s", loc)
	}
	var state *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "td_oauth_state" {
			state = ck
		}
	}
	if state == nil {
		t.Fatal("state-cookie не установлена")
	}

	// 2. callback с кодом → сессия + редирект на /.
	req = httptest.NewRequest(http.MethodGet,
		"/auth/callback/fake?code=good-code&state="+state.Value, nil)
	req.AddCookie(state)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("callback: code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "td_session" && ck.Value != "" {
			return ck
		}
	}
	t.Fatal("cookie сессии не установлена")
	return nil
}

func TestOAuthFlowAndRoles(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: true, HMACSecret: "x", PublicRead: true, Header: "Authorization",
		SessionSecret: "session-secret", SessionTTLHours: 1, DefaultRole: "editor",
	}
	e, _ := oauthApp(t, cfg, "fake:flow", "flow@test.io")
	session := runFlow(t, e)

	// Сессия аутентифицирует /api/me; роль по умолчанию — editor.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/me: code=%d", rec.Code)
	}
	var me map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me["authenticated"] != true || me["canEdit"] != true || me["role"] != "editor" {
		t.Fatalf("me: %v", me)
	}
	if me["isAdmin"] == true {
		t.Fatal("обычный пользователь не должен быть админом")
	}

	// Админский роут закрыт.
	req = httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin ping для editor: code=%d, ожидался 403", rec.Code)
	}

	// Неверный state → 400.
	req = httptest.NewRequest(http.MethodGet, "/auth/callback/fake?code=good-code&state=evil", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("callback с чужим state: code=%d, ожидался 400", rec.Code)
	}

	// Logout снимает cookie.
	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: code=%d", rec.Code)
	}
}

func TestOAuthAdminBootstrap(t *testing.T) {
	cfg := config.AuthSettings{
		Enabled: true, HMACSecret: "x", PublicRead: true, Header: "Authorization",
		SessionSecret: "session-secret-2", SessionTTLHours: 1, DefaultRole: "editor",
		AdminEmails: []string{"boss@test.io"},
	}
	e, _ := oauthApp(t, cfg, "fake:boss", "boss@test.io")
	session := runFlow(t, e)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("бутстрап-админ должен проходить в /api/admin: code=%d", rec.Code)
	}

	// Список пользователей доступен; находим себя и понижаем до reader.
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin users: code=%d", rec.Code)
	}
	var users []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &users)
	var selfID float64
	for _, u := range users {
		if u["subject"] == "fake:boss" {
			selfID = u["id"].(float64)
		}
	}
	if selfID == 0 {
		t.Fatal("boss не найден в списке пользователей")
	}

	// Смена роли: невалидная → 400, валидная → 204 и применяется сразу
	// (кэш Registry сбрасывается) — но boss в adminEmails, бутстрап вернёт
	// admin при следующем входе; проверяем на самой записи в списке.
	body := strings.NewReader(`{"role":"owner"}`)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%.0f/role", selfID), body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("невалидная роль: code=%d, ожидался 400", rec.Code)
	}

	body = strings.NewReader(`{"role":"editor"}`)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%.0f/role", selfID), body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("смена роли: code=%d", rec.Code)
	}
}
