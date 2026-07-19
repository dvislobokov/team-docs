-- name: UpsertUser :one
-- Регистрирует/обновляет пользователя по subject из JWT/OAuth (или "dev").
-- Роль задаётся только при создании (defaultRole) — назначенную админом
-- роль upsert не перетирает. Вызывается из auth-middleware (с кэшем).
INSERT INTO users (subject, username, name, email, role)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (subject) DO UPDATE
SET username     = EXCLUDED.username,
    name         = EXCLUDED.name,
    email        = EXCLUDED.email,
    last_seen_at = now()
RETURNING id, subject, username, name, email, role, created_at, last_seen_at;

-- name: SetUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: ListUsers :many
SELECT id, subject, username, name, email, role, created_at, last_seen_at
FROM users
ORDER BY name, id;
