-- name: UpdateWorkspacesLastPulledAt :exec
; -- @no-track
UPDATE workspace SET last_pulled_at = :last_pulled_at WHERE id IN (sqlc.slice('workspace_ids'));
