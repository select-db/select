-- name: RevokeAPIKey :exec
UPDATE auth.api_key SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
