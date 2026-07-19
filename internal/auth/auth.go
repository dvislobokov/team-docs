package auth

import (
	"errors"
	"fmt"
	"sync"

	"github.com/golang-jwt/jwt/v5"

	"team-docs/internal/config"
)

// Authenticator проверяет JWT согласно конфигу (JWKS/RS256 либо HMAC/HS256)
// и обслуживает встроенные cookie-сессии (OAuth-логин).
type Authenticator struct {
	cfg  config.AuthSettings
	jwks *jwksCache

	randOnce   sync.Once
	randSecret []byte
}

// New создаёт Authenticator. При Enabled=true требуется JWKS-URL или HMAC-секрет.
func New(cfg config.AuthSettings) (*Authenticator, error) {
	a := &Authenticator{cfg: cfg}
	if cfg.Enabled {
		if cfg.JWKSURL == "" && cfg.HMACSecret == "" {
			return nil, errors.New("auth enabled, but neither auth.jwksUrl nor auth.hmacSecret is set")
		}
		if cfg.JWKSURL != "" {
			a.jwks = newJWKSCache(cfg.JWKSURL)
		}
	}
	return a, nil
}

// Enabled сообщает, включена ли проверка авторизации.
func (a *Authenticator) Enabled() bool { return a.cfg.Enabled }

// PublicRead — разрешено ли анонимное чтение (GET) при включённой авторизации.
func (a *Authenticator) PublicRead() bool { return a.cfg.PublicRead }

// CanEdit сообщает, вправе ли пользователь изменять контент (запись).
// Приоритет: EditorGroups (IAM-режим, группы из токена) → роль из БД
// (встроенный режим: reader не пишет, editor/admin пишут). Пустая роль
// (Registry недоступен) трактуется как editor — прежнее поведение.
func (a *Authenticator) CanEdit(u *User) bool {
	if u == nil {
		return false
	}
	if len(a.cfg.EditorGroups) > 0 {
		for _, g := range u.Groups {
			for _, e := range a.cfg.EditorGroups {
				if g == e {
					return true
				}
			}
		}
		return false
	}
	return u.Role != RoleReader
}

// IsAdmin — гард административных операций. В открытом режиме (auth off)
// админка доступна всем: локальная разработка.
func (a *Authenticator) IsAdmin(u *User) bool {
	if !a.Enabled() {
		return true
	}
	return u != nil && u.Role == RoleAdmin
}

// keyfunc выбирает ключ по алгоритму токена (HMAC-секрет или RSA-ключ из JWKS).
func (a *Authenticator) keyfunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
		if a.cfg.HMACSecret == "" {
			return nil, errors.New("HMAC-signed token, but auth.hmacSecret is not configured")
		}
		return []byte(a.cfg.HMACSecret), nil
	}
	if a.jwks != nil {
		return a.jwks.keyfunc(t)
	}
	return nil, fmt.Errorf("unsupported signing method %v", t.Header["alg"])
}

// Verify проверяет подпись и claims токена и возвращает identity.
func (a *Authenticator) Verify(tokenStr string) (*User, error) {
	opts := []jwt.ParserOption{jwt.WithExpirationRequired()}
	if a.cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(a.cfg.Issuer))
	}
	if a.cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(a.cfg.Audience))
	}

	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, a.keyfunc, opts...)
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return a.userFromClaims(claims), nil
}

func (a *Authenticator) userFromClaims(c jwt.MapClaims) *User {
	str := func(k string) string { v, _ := c[k].(string); return v }

	u := &User{
		Subject:  str("sub"),
		Username: str(a.cfg.UsernameClaim),
		Name:     str(a.cfg.NameClaim),
		Email:    str(a.cfg.EmailClaim),
	}
	if u.Username == "" {
		u.Username = u.Subject
	}
	if u.Name == "" {
		u.Name = u.Username
	}

	// Группы: сначала claim "groups", затем Keycloak realm_access.roles.
	if g, ok := c["groups"].([]any); ok {
		for _, x := range g {
			if s, ok := x.(string); ok {
				u.Groups = append(u.Groups, s)
			}
		}
	} else if ra, ok := c["realm_access"].(map[string]any); ok {
		if roles, ok := ra["roles"].([]any); ok {
			for _, x := range roles {
				if s, ok := x.(string); ok {
					u.Groups = append(u.Groups, s)
				}
			}
		}
	}
	return u
}

// DevUser — identity в открытом (dev) режиме, когда авторизация выключена.
func (a *Authenticator) DevUser() *User {
	name := a.cfg.DevUser
	if name == "" {
		name = "Разработчик"
	}
	return &User{Subject: "dev", Username: "dev", Name: name}
}
