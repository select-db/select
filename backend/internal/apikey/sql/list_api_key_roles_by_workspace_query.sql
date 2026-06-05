-- name: ListAPIKeyRolesByWorkspace :many
SELECT atr.api_key_id, atr.role_id, r.name AS role_name
FROM auth.api_key_to_role atr
JOIN auth.api_key k ON k.id = atr.api_key_id
JOIN app.role r ON r.id = atr.role_id
WHERE k.workspace_id = $1 AND k.deleted_at IS NULL AND r.deleted_at IS NULL;
