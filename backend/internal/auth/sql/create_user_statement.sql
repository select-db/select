-- name: CreateUser :one
INSERT INTO app."user" (email, name, github_id)
VALUES ($1, $2, $3)
RETURNING id, email, name;