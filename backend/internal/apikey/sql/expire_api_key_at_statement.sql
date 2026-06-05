-- name: ExpireAPIKeyAt :exec
UPDATE auth.api_key SET expires_at = $2
WHERE id = $1 AND deleted_at IS NULL
  AND (expires_at IS NULL OR expires_at > $2);
