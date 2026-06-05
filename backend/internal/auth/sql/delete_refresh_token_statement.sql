-- name: DeleteRefreshToken :exec
DELETE FROM auth.refresh_token
WHERE hashed_token = $1;