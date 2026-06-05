-- name: SetWorkspaceDeletedAt :exec
UPDATE app.workspace
SET deleted_at = now(), updated_at = now()
WHERE id = $1;
