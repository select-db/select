-- name: ListAPIKeysByWorkspace :many
SELECT id, name, prefix, created_by, expires_at, last_used_at, created_at
FROM auth.api_key
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;
