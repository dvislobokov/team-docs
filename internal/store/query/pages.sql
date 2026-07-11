-- name: GetPageTree :many
-- Плоский список для построения дерева в сайдбаре.
SELECT id, parent_id, title, icon, position
FROM pages
ORDER BY parent_id NULLS FIRST, position, id;

-- name: GetPage :one
SELECT id, parent_id, title, icon, content, position, version, created_at, updated_at
FROM pages
WHERE id = $1;

-- name: CreatePage :one
INSERT INTO pages (parent_id, title, position)
VALUES ($1, $2, COALESCE(
    (SELECT MAX(position) + 1 FROM pages WHERE parent_id IS NOT DISTINCT FROM $1),
    0
))
RETURNING id, parent_id, title, icon, content, position, version, created_at, updated_at;

-- name: UpdatePage :one
-- Optimistic lock: обновит строку только если version совпадает.
-- Если вернулось 0 строк — конфликт версий (кто-то успел сохранить).
UPDATE pages
SET title        = $2,
    content      = $3,
    content_text = $4,
    icon         = $6,
    version      = version + 1,
    updated_at   = now()
WHERE id = $1
  AND version = $5
RETURNING id, parent_id, title, icon, content, position, version, created_at, updated_at;

-- name: MovePage :exec
UPDATE pages
SET parent_id  = $2,
    position   = $3,
    updated_at = now()
WHERE id = $1;

-- name: DeletePage :exec
DELETE FROM pages WHERE id = $1;

-- name: SearchPages :many
SELECT id, parent_id, title, icon,
       ts_headline('simple', content_text, plainto_tsquery('simple', $1),
                   'MaxFragments=1,MaxWords=20,MinWords=5') AS snippet
FROM pages
WHERE to_tsvector('simple', title || ' ' || content_text) @@ plainto_tsquery('simple', $1)
ORDER BY ts_rank(to_tsvector('simple', content_text), plainto_tsquery('simple', $1)) DESC
LIMIT 50;

-- name: InsertRevision :exec
INSERT INTO page_revisions (page_id, version, title, content)
VALUES ($1, $2, $3, $4);

-- name: ListRevisions :many
SELECT id, page_id, version, title, created_at
FROM page_revisions
WHERE page_id = $1
ORDER BY version DESC
LIMIT 100;

-- name: GetRevision :one
SELECT id, page_id, version, title, content, created_at
FROM page_revisions
WHERE id = $1;

-- name: LatestRevisionAt :one
SELECT MAX(created_at)::timestamptz AS last_at
FROM page_revisions
WHERE page_id = $1;
