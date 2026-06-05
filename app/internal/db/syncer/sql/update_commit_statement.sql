-- name: UpdateCommit :one
; -- @no-track
UPDATE
    mutation_commit
SET
    payload = :payload,
    created_at = CURRENT_TIMESTAMP
WHERE
    id = :id
RETURNING *;