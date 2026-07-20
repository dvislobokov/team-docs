package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/config"
	"team-docs/internal/store"
)

// registryTTL — как долго не повторять upsert для одного subject.
const registryTTL = 5 * time.Minute

// Registry сопоставляет identity из токена/сессии со строкой в users: upsert
// по subject c кэшем, чтобы не ходить в БД на каждый запрос. Заодно применяет
// роль по умолчанию и бутстрап администраторов (auth.adminEmails).
type Registry struct {
	q           *store.Queries
	defaultRole string
	admins      map[string]bool // email или subject в нижнем регистре
	mu          sync.RWMutex
	m           map[string]registryEntry
}

type registryEntry struct {
	id      int64
	role    string
	expires time.Time
}

func NewRegistry(pool *pgxpool.Pool, cfg config.AuthSettings) *Registry {
	role := cfg.DefaultRole
	if role != RoleReader && role != RoleEditor && role != RoleAdmin {
		role = RoleEditor
	}
	admins := map[string]bool{}
	for _, a := range cfg.AdminEmails {
		admins[strings.ToLower(strings.TrimSpace(a))] = true
	}
	return &Registry{q: store.New(pool), defaultRole: role, admins: admins, m: map[string]registryEntry{}}
}

// Reset сбрасывает кэш subject→id. Вызывается после импорта бэкапа:
// таблица users пересоздана, старые id невалидны.
func (r *Registry) Reset() {
	r.mu.Lock()
	r.m = map[string]registryEntry{}
	r.mu.Unlock()
}

func (r *Registry) isBootstrapAdmin(u *User) bool {
	return r.admins[strings.ToLower(u.Email)] || r.admins[strings.ToLower(u.Subject)]
}

// EnsureRole — EnsureUser + принудительная роль (локальный админ,
// ldap.adminGroups). Понижение не делает: только повышает до want.
func (r *Registry) EnsureRole(ctx context.Context, u *User, want string) error {
	id, role, err := r.EnsureUser(ctx, u)
	if err != nil {
		return err
	}
	if role == want || (want == RoleAdmin && role == RoleAdmin) || roleRank(role) >= roleRank(want) {
		u.ID, u.Role = id, role
		return nil
	}
	if err := r.q.SetUserRole(ctx, store.SetUserRoleParams{ID: id, Role: want}); err != nil {
		return err
	}
	r.mu.Lock()
	r.m[u.Subject] = registryEntry{id: id, role: want, expires: time.Now().Add(registryTTL)}
	r.mu.Unlock()
	u.ID, u.Role = id, want
	return nil
}

func roleRank(r string) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleReader:
		return 1
	}
	return 0
}

// EnsureUser возвращает id и роль пользователя в БД, создавая/обновляя запись
// при необходимости. Профиль обновляется не чаще registryTTL.
func (r *Registry) EnsureUser(ctx context.Context, u *User) (int64, string, error) {
	r.mu.RLock()
	e, ok := r.m[u.Subject]
	r.mu.RUnlock()
	if ok && time.Now().Before(e.expires) {
		return e.id, e.role, nil
	}

	row, err := r.q.UpsertUser(ctx, store.UpsertUserParams{
		Subject:  u.Subject,
		Username: u.Username,
		Name:     u.Name,
		Email:    u.Email,
		Role:     r.defaultRole,
	})
	if err != nil {
		return 0, "", err
	}
	role := row.Role
	// Бутстрап админа: перечисленным в конфиге роль повышается при входе.
	if role != RoleAdmin && r.isBootstrapAdmin(u) {
		if err := r.q.SetUserRole(ctx, store.SetUserRoleParams{ID: row.ID, Role: RoleAdmin}); err != nil {
			return 0, "", err
		}
		role = RoleAdmin
	}

	r.mu.Lock()
	r.m[u.Subject] = registryEntry{id: row.ID, role: role, expires: time.Now().Add(registryTTL)}
	r.mu.Unlock()
	return row.ID, role, nil
}
