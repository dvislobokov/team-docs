-- name: InsertFile :exec
INSERT INTO files (id, page_id, filename, mime, size, content)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetFile :one
SELECT id, page_id, filename, mime, size, content, created_at
FROM files
WHERE id = $1;
