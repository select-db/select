-- name: RenameAPIKey :exec
UPDATE auth.api_key SET name = $2
WHERE id = $1 AND deleted_at IS NULL;
