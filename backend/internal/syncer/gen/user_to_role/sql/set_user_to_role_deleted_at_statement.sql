-- name: SetUserToRoleDeletedAt :exec
UPDATE app.user_to_role
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2;
