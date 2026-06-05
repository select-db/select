-- name: GetAPIKeyByPrefix :one
SELECT id, workspace_id, name, prefix, hashed_key, created_by, expires_at, last_used_at, created_at, deleted_at
FROM auth.api_key
WHERE prefix = $1 AND deleted_at IS NULL;
