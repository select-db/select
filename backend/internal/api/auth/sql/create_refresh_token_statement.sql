-- name: CreateRefreshToken :exec
INSERT INTO auth.refresh_token (
    hashed_token,
    user_id,
    expires_at,
    issued_ip
) VALUES (
    $1, $2, $3, $4
);