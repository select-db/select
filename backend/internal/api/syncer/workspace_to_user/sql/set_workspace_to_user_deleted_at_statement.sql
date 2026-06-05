-- name: SetWorkspaceToUserDeletedAt :exec
UPDATE app.workspace_to_user
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2;
