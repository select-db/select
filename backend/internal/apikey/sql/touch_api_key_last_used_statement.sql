-- name: TouchAPIKeyLastUsed :exec
UPDATE auth.api_key SET last_used_at = now()
WHERE id = $1;
