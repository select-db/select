-- name: GetAPIKeyForWorkspace :one
SELECT id, name, expires_at
FROM auth.api_key
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL;
