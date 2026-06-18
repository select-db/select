-- name: DeleteConfirmedCommits :exec
; -- @no-track
DELETE FROM mutation_commit
WHERE id IN (sqlc.slice('ids'));
