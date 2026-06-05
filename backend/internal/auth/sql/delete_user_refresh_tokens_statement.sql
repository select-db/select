-- name: DeleteUserRefreshTokens :exec
DELETE FROM auth.refresh_token
WHERE user_id = $1;