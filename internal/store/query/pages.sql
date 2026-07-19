-- name: GetPageTree :many
-- Плоский список для построения дерева в сайдбаре (без корзины), по проекту.
SELECT id, parent_id, title, icon, position
FROM pages
WHERE deleted_at IS NULL AND project_id = $1
ORDER BY parent_id NULLS FIRST, position, id;

-- name: GetPage :one
SELECT p.id, p.parent_id, p.title, p.icon, p.content, p.position, p.version,
       p.created_at, p.updated_at,
       u.name AS updated_by_name
FROM pages p
LEFT JOIN users u ON u.id = p.updated_by
WHERE p.id = $1 AND p.deleted_at IS NULL;

-- name: CreatePage :one
INSERT INTO pages (parent_id, title, position, created_by, updated_by, project_id)
VALUES ($1, $2, COALESCE(
    (SELECT MAX(position) + 1 FROM pages
     WHERE parent_id IS NOT DISTINCT FROM $1 AND deleted_at IS NULL),
    0
), sqlc.narg(author_id), sqlc.narg(author_id), sqlc.arg(project_id))
RETURNING id, parent_id, title, icon, content, position, version, created_at, updated_at;

-- name: UpdatePage :one
-- Optimistic lock: обновит строку только если version совпадает.
-- Если вернулось 0 строк — конфликт версий (кто-то успел сохранить).
UPDATE pages
SET title        = $2,
    content      = $3,
    content_text = $4,
    icon         = $6,
    updated_by   = sqlc.narg(author_id),
    version      = version + 1,
    updated_at   = now()
WHERE id = $1
  AND version = $5
  AND deleted_at IS NULL
RETURNING id, parent_id, title, icon, content, position, version, created_at, updated_at;

-- name: GetPageMeta :one
-- Лёгкое чтение для move/проверок: родитель и позиция без контента.
SELECT id, parent_id, position
FROM pages
WHERE id = $1 AND deleted_at IS NULL;

-- Проверка «candidate в поддереве root» для move живёт сырым SQL в
-- internal/pages/handler.go (isInSubtreeSQL): рекурсивный CTE не проходит
-- через анализатор sqlc.

-- name: CountSiblings :one
-- Число детей родителя без самой переносимой страницы (для клампа позиции).
SELECT COUNT(*)
FROM pages
WHERE parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)
  AND id <> sqlc.arg(page_id)
  AND deleted_at IS NULL;

-- name: ShiftAfterRemove :exec
-- «Изъятие» страницы из старого родителя: соседи ниже сдвигаются вверх.
UPDATE pages
SET position = position - 1
WHERE parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)
  AND position > sqlc.arg(position)
  AND id <> sqlc.arg(page_id)
  AND deleted_at IS NULL;

-- name: ShiftForInsert :exec
-- Освобождение места под вставку: соседи с позиции вставки сдвигаются вниз.
UPDATE pages
SET position = position + 1
WHERE parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)
  AND position >= sqlc.arg(position)
  AND id <> sqlc.arg(page_id)
  AND deleted_at IS NULL;

-- name: MovePage :exec
UPDATE pages
SET parent_id  = $2,
    position   = $3,
    updated_at = now()
WHERE id = $1;

-- Мягкое удаление и восстановление поддерева — сырой SQL с рекурсивным CTE
-- в internal/pages/trash.go (анализатор sqlc не понимает рекурсию).

-- name: ListTrash :many
-- Содержимое корзины: корни удалённых поддеревьев (родитель жив или отсутствует).
SELECT p.id, p.title, p.icon, p.deleted_at, p.project_id
FROM pages p
LEFT JOIN pages par ON par.id = p.parent_id
WHERE p.deleted_at IS NOT NULL
  AND (p.parent_id IS NULL OR par.deleted_at IS NULL)
ORDER BY p.deleted_at DESC;

-- name: PurgePage :execrows
-- Окончательное удаление из корзины (FK-каскад добивает поддерево).
DELETE FROM pages WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: PurgeExpired :execrows
-- Автоочистка корзины: всё, что удалено раньше отсечки.
DELETE FROM pages WHERE deleted_at IS NOT NULL AND deleted_at < $1;

-- name: SearchPages :many
-- Поиск по генерируемой колонке search_vector (русская морфология, GIN-индекс).
SELECT id, parent_id, title, icon,
       ts_headline('russian', content_text, plainto_tsquery('russian', $1),
                   'MaxFragments=1,MaxWords=20,MinWords=5') AS snippet
FROM pages
WHERE search_vector @@ plainto_tsquery('russian', $1)
  AND deleted_at IS NULL
  AND project_id = ANY(sqlc.arg(project_ids)::bigint[])
ORDER BY ts_rank(search_vector, plainto_tsquery('russian', $1)) DESC
LIMIT 50;

-- name: InsertRevision :exec
INSERT INTO page_revisions (page_id, version, title, content, author_id)
VALUES ($1, $2, $3, $4, sqlc.narg(author_id));

-- name: ListRevisions :many
SELECT r.id, r.page_id, r.version, r.title, r.created_at,
       u.name AS author_name
FROM page_revisions r
LEFT JOIN users u ON u.id = r.author_id
WHERE r.page_id = $1
ORDER BY r.version DESC
LIMIT 100;

-- name: GetRevision :one
SELECT id, page_id, version, title, content, created_at
FROM page_revisions
WHERE id = $1;

-- name: LatestRevisionAt :one
SELECT MAX(created_at)::timestamptz AS last_at
FROM page_revisions
WHERE page_id = $1;
