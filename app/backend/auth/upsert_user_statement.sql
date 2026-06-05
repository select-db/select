-- name: UpsertUser :one
; -- @no-track
INSERT INTO user (id, name)
VALUES (:id, :name)
ON CONFLICT (id) DO UPDATE 
SET
  name = EXCLUDED.name
RETURNING *;