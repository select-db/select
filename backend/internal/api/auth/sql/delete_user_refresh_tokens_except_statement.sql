-- name: DeleteUserRefreshTokensExcept :exec
DELETE FROM auth.refresh_token
WHERE user_id = $1 AND hashed_token != $2;
