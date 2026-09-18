-- name: DeleteExpiredUserRefreshTokens :exec
DELETE FROM auth.refresh_token
WHERE user_id = $1 AND expires_at < now();
