-- name: GetRefreshToken :one
SELECT * FROM auth.refresh_token
WHERE hashed_token = $1 AND user_id = $2;