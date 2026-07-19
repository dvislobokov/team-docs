-- name: ListGroups :many
SELECT g.id, g.name, g.created_at, COUNT(gm.user_id) AS member_count
FROM groups g
LEFT JOIN group_members gm ON gm.group_id = g.id
GROUP BY g.id
ORDER BY g.name;

-- name: CreateGroup :one
INSERT INTO groups (name) VALUES ($1) RETURNING *;

-- name: DeleteGroup :execrows
DELETE FROM groups WHERE id = $1;

-- name: ListGroupMembers :many
SELECT u.id, u.name, u.username, u.email
FROM group_members gm
JOIN users u ON u.id = gm.user_id
WHERE gm.group_id = $1
ORDER BY u.name, u.id;

-- name: AddGroupMember :exec
INSERT INTO group_members (group_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveGroupMember :exec
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- name: GetProjectGroupRole :one
-- Лучшая роль пользователя в проекте через его группы.
SELECT pgm.role
FROM project_group_members pgm
JOIN group_members gm ON gm.group_id = pgm.group_id
WHERE pgm.project_id = $1 AND gm.user_id = $2
ORDER BY CASE pgm.role WHEN 'admin' THEN 3 WHEN 'editor' THEN 2 ELSE 1 END DESC
LIMIT 1;

-- name: ListProjectGroups :many
SELECT pgm.group_id, pgm.role, g.name
FROM project_group_members pgm
JOIN groups g ON g.id = pgm.group_id
WHERE pgm.project_id = $1
ORDER BY g.name;

-- name: UpsertProjectGroup :exec
INSERT INTO project_group_members (project_id, group_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, group_id) DO UPDATE SET role = EXCLUDED.role;

-- name: RemoveProjectGroup :exec
DELETE FROM project_group_members WHERE project_id = $1 AND group_id = $2;
