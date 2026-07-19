-- name: ListProjects :many
SELECT *
FROM projects
ORDER BY id;

-- name: GetProjectByKey :one
SELECT *
FROM projects
WHERE key = $1;

-- name: GetProject :one
SELECT *
FROM projects
WHERE id = $1;

-- name: CreateProject :one
INSERT INTO projects (key, name, icon, visibility)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, icon = $3, visibility = $4
WHERE id = $1
RETURNING *;

-- name: CountProjectPages :one
SELECT COUNT(*) FROM pages WHERE project_id = $1;

-- name: DeleteProject :execrows
-- Дефолтный проект 'main' удалить нельзя; страницы держит FK RESTRICT.
DELETE FROM projects WHERE id = $1 AND key <> 'main';

-- name: ListProjectMembers :many
SELECT m.user_id, m.role, m.added_at, u.name, u.username, u.email
FROM project_members m
JOIN users u ON u.id = m.user_id
WHERE m.project_id = $1
ORDER BY u.name, u.id;

-- name: GetProjectMemberRole :one
SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: UpsertProjectMember :exec
INSERT INTO project_members (project_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role;

-- name: RemoveProjectMember :exec
DELETE FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: GetPageProject :one
-- Проект страницы (включая корзину — гарды нужны и для restore/purge).
SELECT project_id FROM pages WHERE id = $1;
