-- name: ListFavorites :many
-- Избранное пользователя с метаданными страниц; удалённые и шаблоны
-- отфильтрованы, доступность проекта проверяет хендлер.
SELECT f.page_id, f.created_at, p.title, p.icon, p.project_id
FROM favorites f
JOIN pages p ON p.id = f.page_id
WHERE f.user_id = $1
  AND p.deleted_at IS NULL
  AND NOT p.is_template
ORDER BY f.created_at DESC, f.page_id;

-- name: AddFavorite :exec
INSERT INTO favorites (user_id, page_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM favorites
WHERE user_id = $1 AND page_id = $2;
