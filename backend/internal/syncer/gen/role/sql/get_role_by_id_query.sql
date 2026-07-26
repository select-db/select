-- name: GetRoleByID :one
SELECT id, workspace_id, name, updated_at, deleted_at
FROM app.role
WHERE id = $1 AND workspace_id = $2;
