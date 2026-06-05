-- name: GetLastPulledAtForWorkspace :one
SELECT last_pulled_at FROM workspace WHERE id = :workspace_id;
