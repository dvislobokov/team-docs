package auth

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/store"
)

// registryTTL — как долго не повторять upsert для одного subject.
const registryTTL = 5 * time.Minute

// Registry сопоставляет identity из токена с строкой в users: upsert по
// subject c кэшем, чтобы не ходить в БД на каждый запрос.
type Registry struct {
	q  *store.Queries
	mu sync.RWMutex
	m  map[string]registryEntry
}

type registryEntry struct {
	id      int64
	expires time.Time
}

func NewRegistry(pool *pgxpool.Pool) *Registry {
	return &Registry{q: store.New(pool), m: map[string]registryEntry{}}
}

// Reset сбрасывает кэш subject→id. Вызывается после импорта бэкапа:
// таблица users пересоздана, старые id невалидны.
func (r *Registry) Reset() {
	r.mu.Lock()
	r.m = map[string]registryEntry{}
	r.mu.Unlock()
}

// EnsureUser возвращает id пользователя в БД, создавая/обновляя запись при
// необходимости. Профиль (имя/почта) обновляется не чаще registryTTL.
func (r *Registry) EnsureUser(ctx context.Context, u *User) (int64, error) {
	r.mu.RLock()
	e, ok := r.m[u.Subject]
	r.mu.RUnlock()
	if ok && time.Now().Before(e.expires) {
		return e.id, nil
	}

	row, err := r.q.UpsertUser(ctx, store.UpsertUserParams{
		Subject:  u.Subject,
		Username: u.Username,
		Name:     u.Name,
		Email:    u.Email,
	})
	if err != nil {
		return 0, err
	}

	r.mu.Lock()
	r.m[u.Subject] = registryEntry{id: row.ID, expires: time.Now().Add(registryTTL)}
	r.mu.Unlock()
	return row.ID, nil
}
