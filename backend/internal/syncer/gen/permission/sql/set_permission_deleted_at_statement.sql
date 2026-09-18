-- name: SetPermissionDeletedAt :exec
UPDATE app.permission
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2;
