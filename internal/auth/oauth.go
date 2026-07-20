package auth

// Встроенный OAuth2-логин (ROADMAP §8): Google, Yandex, VK, Apple.
// Классический authorization code flow без внешних библиотек: провайдеры
// описываются URL-ами и функцией извлечения профиля, что позволяет
// подставлять фейковый провайдер в тестах.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"team-docs/internal/config"
)

const stateCookie = "td_oauth_state"

// Provider — описание OAuth-провайдера.
type Provider struct {
	Key      string
	Label    string
	AuthURL  string
	TokenURL string
	Scopes   []string
	ClientID string
	// clientSecret возвращает секрет (у Apple — генерируемый ES256-JWT).
	clientSecret func() (string, error)
	// profile извлекает identity из token-ответа провайдера.
	profile func(client *http.Client, tokenResp map[string]any) (*User, error)
	// extraAuthParams — доп. параметры авторизационного URL (Apple: form_post).
	extraAuthParams map[string]string
	// postCallback — Apple шлёт callback POST-ом (response_mode=form_post).
	postCallback bool
	// discover — ленивое OIDC-discovery (generic-провайдер): заполняет
	// AuthURL/TokenURL при первом обращении; при ошибке повторяется.
	discover   func(client *http.Client) error
	discoverMu sync.Mutex
	discovered bool
}

// ready выполняет отложенное discovery провайдера (с повтором при ошибке).
func (h *OAuthHandler) ready(p *Provider) error {
	if p.discover == nil {
		return nil
	}
	p.discoverMu.Lock()
	defer p.discoverMu.Unlock()
	if p.discovered {
		return nil
	}
	if err := p.discover(h.client); err != nil {
		return err
	}
	p.discovered = true
	return nil
}

// OAuthHandler обслуживает /auth/*.
type OAuthHandler struct {
	a         *Authenticator
	reg       *Registry
	log       *srog.Logger
	publicURL string
	providers map[string]*Provider
	order     []string
	client    *http.Client
	// passwordEnabled — показывать ли форму логин/пароль (LDAP/локальный админ).
	passwordEnabled func() bool
}

// SetPasswordEnabled подключает индикатор формы логин/пароль к /auth/providers.
func (h *OAuthHandler) SetPasswordEnabled(fn func() bool) { h.passwordEnabled = fn }

func NewOAuthHandler(a *Authenticator, reg *Registry, log *srog.Logger, publicURL string, providers []*Provider) *OAuthHandler {
	h := &OAuthHandler{
		a: a, reg: reg, log: log,
		publicURL: strings.TrimRight(publicURL, "/"),
		providers: map[string]*Provider{},
		client:    &http.Client{Timeout: 10 * time.Second},
	}
	for _, p := range providers {
		h.providers[p.Key] = p
		h.order = append(h.order, p.Key)
	}
	return h
}

// Register вешает роуты на корневой echo (вне /api и auth-middleware).
func (h *OAuthHandler) Register(e *echo.Echo) {
	e.GET("/auth/providers", h.listProviders)
	e.GET("/auth/login/:key", h.login)
	e.Any("/auth/callback/:key", h.callback)
	e.POST("/auth/logout", h.logout)
}

func (h *OAuthHandler) listProviders(c echo.Context) error {
	type item struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	providers := make([]item, 0, len(h.order))
	for _, k := range h.order {
		providers = append(providers, item{Key: k, Label: h.providers[k].Label})
	}
	password := h.passwordEnabled != nil && h.passwordEnabled()
	return c.JSON(http.StatusOK, echo.Map{"providers": providers, "password": password})
}

func (h *OAuthHandler) redirectURI(p *Provider) string {
	return h.publicURL + "/auth/callback/" + p.Key
}

func (h *OAuthHandler) login(c echo.Context) error {
	p, ok := h.providers[c.Param("key")]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "unknown provider")
	}
	if err := h.ready(p); err != nil {
		h.log.Error(err, "oauth: discovery failed for {Provider}", p.Key)
		return echo.NewHTTPError(http.StatusBadGateway, "провайдер недоступен")
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	state := hex.EncodeToString(buf)
	c.SetCookie(&http.Cookie{
		Name: stateCookie, Value: state, Path: "/auth", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})

	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", h.redirectURI(p))
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(p.Scopes) > 0 {
		q.Set("scope", strings.Join(p.Scopes, " "))
	}
	for k, v := range p.extraAuthParams {
		q.Set(k, v)
	}
	return c.Redirect(http.StatusFound, p.AuthURL+"?"+q.Encode())
}

func (h *OAuthHandler) callback(c echo.Context) error {
	p, ok := h.providers[c.Param("key")]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "unknown provider")
	}
	// Apple присылает POST form; остальные — GET query.
	code := c.QueryParam("code")
	state := c.QueryParam("state")
	if p.postCallback && code == "" {
		code = c.FormValue("code")
		state = c.FormValue("state")
	}
	ck, err := c.Cookie(stateCookie)
	if err != nil || ck.Value == "" || ck.Value != state {
		return echo.NewHTTPError(http.StatusBadRequest, "oauth state mismatch")
	}
	c.SetCookie(&http.Cookie{Name: stateCookie, Value: "", Path: "/auth", MaxAge: -1})
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing authorization code")
	}

	if err := h.ready(p); err != nil {
		h.log.Error(err, "oauth: discovery failed for {Provider}", p.Key)
		return echo.NewHTTPError(http.StatusBadGateway, "провайдер недоступен")
	}
	tokenResp, err := h.exchange(p, code)
	if err != nil {
		h.log.Error(err, "oauth: code exchange failed for {Provider}", p.Key)
		return echo.NewHTTPError(http.StatusBadGateway, "провайдер не подтвердил вход")
	}
	u, err := p.profile(h.client, tokenResp)
	if err != nil {
		h.log.Error(err, "oauth: profile fetch failed for {Provider}", p.Key)
		return echo.NewHTTPError(http.StatusBadGateway, "не удалось получить профиль")
	}

	if _, _, err := h.reg.EnsureUser(c.Request().Context(), u); err != nil {
		h.log.Error(err, "oauth: user upsert failed for {Subject}", u.Subject)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if err := h.a.IssueSession(c, u); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.Redirect(http.StatusFound, "/")
}

func (h *OAuthHandler) logout(c echo.Context) error {
	h.a.ClearSession(c)
	return c.NoContent(http.StatusNoContent)
}

// exchange меняет code на token-ответ провайдера.
func (h *OAuthHandler) exchange(p *Provider, code string) (map[string]any, error) {
	secret, err := p.clientSecret()
	if err != nil {
		return nil, fmt.Errorf("client secret: %w", err)
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", secret)
	form.Set("redirect_uri", h.redirectURI(p))

	resp, err := h.client.PostForm(p.TokenURL, form)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("token response: %w", err)
	}
	return out, nil
}

// --- Сборка провайдеров из конфига ---

// BuildProviders возвращает настроенных провайдеров (по clientId).
func BuildProviders(cfg config.AuthSettings) []*Provider {
	var out []*Provider
	if g := cfg.Providers.Google; g.ClientID != "" {
		out = append(out, googleProvider(g))
	}
	if y := cfg.Providers.Yandex; y.ClientID != "" {
		out = append(out, yandexProvider(y))
	}
	if v := cfg.Providers.VK; v.ClientID != "" {
		out = append(out, vkProvider(v))
	}
	if ap := cfg.Providers.Apple; ap.ClientID != "" {
		out = append(out, appleProvider(ap))
	}
	if o := cfg.Providers.OIDC; o.ClientID != "" && o.Issuer != "" {
		out = append(out, oidcProvider(o))
	}
	return out
}

// oidcProvider — generic OpenID Connect (Keycloak, Authentik, Dex, …):
// эндпоинты берутся из discovery, профиль — из userinfo. Группы (claim
// groupsClaim либо Keycloak realm_access.roles) кладутся в User.Groups и
// далее в сессию — editorGroups работает как в proxy-режиме.
func oidcProvider(c config.OIDCClientSettings) *Provider {
	label := c.Label
	if label == "" {
		label = "SSO"
	}
	var userinfoURL string
	p := &Provider{
		Key: "oidc", Label: label,
		Scopes:   []string{"openid", "profile", "email"},
		ClientID: c.ClientID, clientSecret: staticSecret(config.SecretString(c.ClientSecret)),
	}
	p.discover = func(client *http.Client) error {
		doc, err := fetchJSON(client,
			strings.TrimRight(c.Issuer, "/")+"/.well-known/openid-configuration", "")
		if err != nil {
			return fmt.Errorf("oidc discovery: %w", err)
		}
		p.AuthURL = str(doc, "authorization_endpoint")
		p.TokenURL = str(doc, "token_endpoint")
		userinfoURL = str(doc, "userinfo_endpoint")
		if p.AuthURL == "" || p.TokenURL == "" || userinfoURL == "" {
			return fmt.Errorf("oidc discovery: неполный ответ %s", c.Issuer)
		}
		return nil
	}
	p.profile = func(client *http.Client, tok map[string]any) (*User, error) {
		info, err := fetchJSON(client, userinfoURL, "Bearer "+str(tok, "access_token"))
		if err != nil {
			return nil, err
		}
		u := &User{
			Subject:  "oidc:" + str(info, "sub"),
			Username: str(info, "preferred_username"),
			Name:     str(info, "name"),
			Email:    str(info, "email"),
		}
		if u.Username == "" {
			u.Username = u.Email
		}
		if u.Name == "" {
			u.Name = u.Username
		}
		claim := c.GroupsClaim
		if claim == "" {
			claim = "groups"
		}
		if g, ok := info[claim].([]any); ok {
			for _, x := range g {
				if s, ok := x.(string); ok {
					u.Groups = append(u.Groups, s)
				}
			}
		}
		if ra, ok := info["realm_access"].(map[string]any); ok { // Keycloak
			if roles, ok := ra["roles"].([]any); ok {
				for _, x := range roles {
					if s, ok := x.(string); ok {
						u.Groups = append(u.Groups, s)
					}
				}
			}
		}
		return u, nil
	}
	return p
}

func staticSecret(s string) func() (string, error) {
	return func() (string, error) { return s, nil }
}

// fetchJSON делает GET с заголовком авторизации и парсит JSON.
func fetchJSON(client *http.Client, rawURL, authHeader string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo %d: %s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func str(m map[string]any, k string) string { v, _ := m[k].(string); return v }

func googleProvider(c config.OAuthClientSettings) *Provider {
	return &Provider{
		Key: "google", Label: "Google",
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		Scopes:   []string{"openid", "email", "profile"},
		ClientID: c.ClientID, clientSecret: staticSecret(config.SecretString(c.ClientSecret)),
		profile: func(client *http.Client, tok map[string]any) (*User, error) {
			info, err := fetchJSON(client,
				"https://openidconnect.googleapis.com/v1/userinfo",
				"Bearer "+str(tok, "access_token"))
			if err != nil {
				return nil, err
			}
			return &User{
				Subject:  "google:" + str(info, "sub"),
				Username: str(info, "email"),
				Name:     str(info, "name"),
				Email:    str(info, "email"),
			}, nil
		},
	}
}

func yandexProvider(c config.OAuthClientSettings) *Provider {
	return &Provider{
		Key: "yandex", Label: "Яндекс",
		AuthURL:  "https://oauth.yandex.ru/authorize",
		TokenURL: "https://oauth.yandex.ru/token",
		ClientID: c.ClientID, clientSecret: staticSecret(config.SecretString(c.ClientSecret)),
		profile: func(client *http.Client, tok map[string]any) (*User, error) {
			info, err := fetchJSON(client,
				"https://login.yandex.ru/info?format=json",
				"OAuth "+str(tok, "access_token"))
			if err != nil {
				return nil, err
			}
			name := str(info, "display_name")
			if name == "" {
				name = str(info, "login")
			}
			return &User{
				Subject:  "yandex:" + str(info, "id"),
				Username: str(info, "login"),
				Name:     name,
				Email:    str(info, "default_email"),
			}, nil
		},
	}
}

func vkProvider(c config.OAuthClientSettings) *Provider {
	return &Provider{
		Key: "vk", Label: "VK",
		AuthURL:  "https://oauth.vk.com/authorize",
		TokenURL: "https://oauth.vk.com/access_token",
		Scopes:   []string{"email"},
		ClientID: c.ClientID, clientSecret: staticSecret(config.SecretString(c.ClientSecret)),
		profile: func(client *http.Client, tok map[string]any) (*User, error) {
			// VK кладёт user_id и email прямо в token-ответ.
			uid := fmt.Sprintf("%v", tok["user_id"])
			if uid == "" || uid == "<nil>" {
				return nil, fmt.Errorf("vk: user_id отсутствует в token-ответе")
			}
			name := "VK " + uid
			if info, err := fetchJSON(client,
				"https://api.vk.com/method/users.get?v=5.199&access_token="+
					url.QueryEscape(str(tok, "access_token")), ""); err == nil {
				if arr, ok := info["response"].([]any); ok && len(arr) > 0 {
					if m, ok := arr[0].(map[string]any); ok {
						name = strings.TrimSpace(str(m, "first_name") + " " + str(m, "last_name"))
					}
				}
			}
			return &User{
				Subject:  "vk:" + uid,
				Username: uid,
				Name:     name,
				Email:    str(tok, "email"),
			}, nil
		},
	}
}

func appleProvider(c config.AppleClientSettings) *Provider {
	return &Provider{
		Key: "apple", Label: "Apple",
		AuthURL:      "https://appleid.apple.com/auth/authorize",
		TokenURL:     "https://appleid.apple.com/auth/token",
		Scopes:       []string{"email"},
		ClientID:     c.ClientID,
		clientSecret: func() (string, error) { return appleClientSecret(c) },
		// Apple при запросе scope требует form_post-callback.
		extraAuthParams: map[string]string{"response_mode": "form_post"},
		postCallback:    true,
		profile: func(_ *http.Client, tok map[string]any) (*User, error) {
			// Профиль — claims id_token. Токен получен напрямую от Apple по
			// TLS, поэтому подпись не перепроверяем.
			idt := str(tok, "id_token")
			if idt == "" {
				return nil, fmt.Errorf("apple: id_token отсутствует")
			}
			claims := jwt.MapClaims{}
			if _, _, err := jwt.NewParser().ParseUnverified(idt, claims); err != nil {
				return nil, fmt.Errorf("apple id_token: %w", err)
			}
			sub, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			name := email
			if name == "" {
				name = "Apple " + sub
			}
			return &User{Subject: "apple:" + sub, Username: email, Name: name, Email: email}, nil
		},
	}
}
