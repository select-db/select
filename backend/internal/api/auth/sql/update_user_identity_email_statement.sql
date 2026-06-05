-- name: UpdateUserIdentityEmail :exec
UPDATE app.user_identity
SET email = $3
WHERE provider = $1 AND provider_user_id = $2;
