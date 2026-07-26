-- name: SetGroupToRoleDeletedAt :exec
UPDATE app.group_to_role
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2;
