-- name: UpsertUser :one
-- Регистрирует/обновляет пользователя по subject из JWT (или "dev").
-- Вызывается из auth-middleware (с кэшем, не на каждый запрос).
INSERT INTO users (subject, username, name, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT (subject) DO UPDATE
SET username     = EXCLUDED.username,
    name         = EXCLUDED.name,
    email        = EXCLUDED.email,
    last_seen_at = now()
RETURNING id, subject, username, name, email, created_at, last_seen_at;
