-- name: ReactivateWorkspaceToUser :one
UPDATE app.workspace_to_user
SET deleted_at = NULL, updated_at = NOW()
WHERE workspace_id = $1 AND user_id = $2 AND deleted_at IS NOT NULL
RETURNING id;
