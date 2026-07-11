-- name: CreateDiagram :one
INSERT INTO diagrams (id, page_id, xml)
VALUES ($1, $2, '')
RETURNING id, page_id, xml, updated_at;

-- name: GetDiagram :one
SELECT id, page_id, xml, updated_at
FROM diagrams
WHERE id = $1;

-- name: GetDiagramPreview :one
SELECT preview
FROM diagrams
WHERE id = $1;

-- name: UpdateDiagram :exec
UPDATE diagrams
SET xml        = $2,
    preview    = $3,
    updated_at = now()
WHERE id = $1;
