-- name: CreateWorkspace :one
; -- @no-track
INSERT INTO workspace (id, name, owner_id)
VALUES (:id, :name, :owner_id)
ON CONFLICT (id) DO NOTHING
RETURNING *;