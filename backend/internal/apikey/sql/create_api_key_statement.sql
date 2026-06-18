-- name: CreateAPIKey :one
INSERT INTO auth.api_key (
    workspace_id,
    name,
    prefix,
    hashed_key,
    created_by,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING id, prefix;
