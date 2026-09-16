-- name: GetAPIKeyByPrefix :one
SELECT
  k.id,
  k.workspace_id,
  k.name,
  k.prefix,
  k.hashed_key,
  k.created_by,
  k.expires_at,
  k.last_used_at,
  k.created_at,
  k.deleted_at
FROM
  auth.api_key k
  JOIN app.workspace w ON w.id = k.workspace_id
WHERE
  k.prefix = $1
  AND k.deleted_at IS NULL
  AND w.deleted_at IS NULL;
