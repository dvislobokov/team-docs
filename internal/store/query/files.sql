-- name: InsertFile :exec
INSERT INTO files (id, page_id, filename, mime, size, content)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetFile :one
SELECT id, page_id, filename, mime, size, content, created_at
FROM files
WHERE id = $1;

-- name: FindFileProject :one
-- Проект файла — по живой странице, в контенте которой встречается его UUID.
SELECT project_id
FROM pages
WHERE deleted_at IS NULL
  AND content::text LIKE '%' || sqlc.arg(file_id)::text || '%'
LIMIT 1;
