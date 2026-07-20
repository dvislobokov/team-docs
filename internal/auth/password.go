package auth

// Вход по логину/паролю: break-glass локальный админ (bcrypt-хэш в конфиге)
// и LDAP. POST /auth/password → cookie-сессия (та же, что у OAuth).

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"team-docs/internal/config"
)

// PasswordHandler обслуживает /auth/password.
type PasswordHandler struct {
	a     *Authenticator
	reg   *Registry
	ldap  *LDAPAuthenticator // nil — LDAP не настроен
	local config.LocalAdminSettings
	log   *srog.Logger
}

func NewPasswordHandler(a *Authenticator, reg *Registry, l *LDAPAuthenticator,
	local config.LocalAdminSettings, log *srog.Logger) *PasswordHandler {
	return &PasswordHandler{a: a, reg: reg, ldap: l, local: local, log: log}
}

// Enabled — показывать ли форму логин/пароль на экране входа.
func (h *PasswordHandler) Enabled() bool {
	return h.ldap != nil || (h.local.Username != "" && h.local.PasswordHash != "")
}

func (h *PasswordHandler) Register(e *echo.Echo) {
	e.POST("/auth/password", h.login)
}

func (h *PasswordHandler) login(c echo.Context) error {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	req.Login = strings.TrimSpace(req.Login)

	ctx := c.Request().Context()
	unauthorized := echo.NewHTTPError(http.StatusUnauthorized, "неверный логин или пароль")

	// 1) Локальный админ: если логин совпал — только локальная проверка
	// (никогда не уходим в LDAP с его паролем).
	if h.local.Username != "" && strings.EqualFold(req.Login, h.local.Username) {
		if h.local.PasswordHash == "" ||
			bcrypt.CompareHashAndPassword([]byte(h.local.PasswordHash), []byte(req.Password)) != nil {
			return unauthorized
		}
		u := &User{Subject: "local:" + strings.ToLower(h.local.Username),
			Username: h.local.Username, Name: h.local.Username}
		if err := h.reg.EnsureRole(ctx, u, RoleAdmin); err != nil {
			h.log.Error(err, "auth: local admin upsert failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
		}
		if err := h.a.IssueSession(c, u); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
		}
		return c.NoContent(http.StatusNoContent)
	}

	// 2) LDAP.
	if h.ldap == nil {
		return unauthorized
	}
	u, err := h.ldap.Authenticate(req.Login, req.Password)
	if errors.Is(err, ErrLDAPAuth) {
		return unauthorized
	}
	if err != nil {
		h.log.Error(err, "auth: ldap authenticate failed for {Login}", req.Login)
		return echo.NewHTTPError(http.StatusBadGateway, "каталог недоступен")
	}

	if h.ldap.IsAdminGroupMember(u.Groups) {
		err = h.reg.EnsureRole(ctx, u, RoleAdmin)
	} else {
		u.ID, u.Role, err = h.reg.EnsureUser(ctx, u)
	}
	if err != nil {
		h.log.Error(err, "auth: ldap user upsert failed for {Subject}", u.Subject)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	// Зеркалирование групп (фаза 2): роль в проекте можно выдать LDAP-группе.
	if h.ldap.SyncEnabled() && u.ID != 0 {
		if err := h.reg.SyncLDAPGroups(ctx, u.ID, GroupNames(u.Groups)); err != nil {
			h.log.Error(err, "auth: ldap group sync failed for {Subject}", u.Subject)
		}
	}
	if err := h.a.IssueSession(c, u); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}
