package auth

// Интеграционный тест generic OIDC-провайдера (Keycloak-стиль): discovery,
// обмен кода, userinfo c realm_access.roles, группы в сессии + editorGroups.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"

	"team-docs/internal/config"
)

// fakeKeycloak — discovery + token + userinfo.
func fakeKeycloak(t *testing.T, roles []string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/realms/td/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authorization_endpoint": srv.URL + "/realms/td/auth",
			"token_endpoint":         srv.URL + "/realms/td/token",
			"userinfo_endpoint":      srv.URL + "/realms/td/userinfo",
		})
	})
	mux.HandleFunc("/realms/td/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("code") != "kc-code" {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "kc-token"})
	})
	mux.HandleFunc("/realms/td/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer kc-token" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub": "kc-1", "preferred_username": "kc.user", "name": "Кейклок Юзер",
			"email":        "kc@test.io",
			"realm_access": map[string]any{"roles": roles},
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func oidcFlow(t *testing.T, e *echo.Echo) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/login/oidc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var state *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "td_oauth_state" {
			state = ck
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/auth/callback/oidc?code=kc-code&state="+state.Value, nil)
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

func TestOIDCKeycloakFlowWithEditorGroups(t *testing.T) {
	pool := oauthPool(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(t.Context(), `DELETE FROM users WHERE subject = 'oidc:kc-1'`)
	})

	run := func(roles []string) (*echo.Echo, *http.Cookie) {
		idp := fakeKeycloak(t, roles)
		cfg := config.AuthSettings{
			Enabled: true, HMACSecret: "x", PublicRead: true, Header: "Authorization",
			SessionSecret: "kc-secret", SessionTTLHours: 1, DefaultRole: "editor",
			EditorGroups: []string{"docs-editors"},
			Providers: config.ProvidersSettings{OIDC: config.OIDCClientSettings{
				Label: "Keycloak", Issuer: idp.URL + "/realms/td",
				ClientID: "td", ClientSecret: "cs",
			}},
		}
		a, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		log := srog.NewConsole()
		t.Cleanup(func() { _ = log.Close() })
		reg := NewRegistry(pool, cfg)

		providers := BuildProviders(cfg)
		if len(providers) != 1 || providers[0].Key != "oidc" {
			t.Fatalf("BuildProviders: %v", providers)
		}
		e := echo.New()
		NewOAuthHandler(a, reg, log, "http://app.local", providers).Register(e)
		api := e.Group("/api")
		api.Use(Middleware(a, reg, log))
		api.Use(RequireEditor(a, log))
		api.GET("/probe", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
		api.POST("/probe", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
		return e, oidcFlow(t, e)
	}

	// Роль docs-editors в realm_access → запись разрешена.
	e, session := run([]string{"docs-editors"})
	req := httptest.NewRequest(http.MethodPost, "/api/probe", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("editor из Keycloak-ролей: code=%d, ожидался 204", rec.Code)
	}

	// Без роли — только чтение (группы в сессии → editorGroups отсекает).
	e2, session2 := run([]string{"other-role"})
	req = httptest.NewRequest(http.MethodPost, "/api/probe", nil)
	req.AddCookie(session2)
	rec = httptest.NewRecorder()
	e2.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("без editor-роли: code=%d, ожидался 403", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/probe", nil)
	req.AddCookie(session2)
	rec = httptest.NewRecorder()
	e2.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("чтение без editor-роли: code=%d", rec.Code)
	}
}
