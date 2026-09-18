-- name: GetUserToRoleByID :one
SELECT id, user_id, role_id, workspace_id, updated_at, deleted_at
FROM app.user_to_role
WHERE id = $1 AND workspace_id = $2;
