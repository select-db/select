-- name: GetWorkspaceToUserByID :one
SELECT id, workspace_id, user_id, updated_at, deleted_at
FROM app.workspace_to_user
WHERE id = $1 AND workspace_id = $2;
