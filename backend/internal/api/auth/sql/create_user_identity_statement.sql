-- name: CreateUserIdentity :exec
INSERT INTO app.user_identity (user_id, provider, provider_user_id, email)
VALUES ($1, $2, $3, $4);
