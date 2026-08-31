-- name: GetWorkspaceByID :one
SELECT id, name, git_remote_url, last_pulled_at, statement_timeout_ms, max_result_size_mb, logo
FROM workspace
WHERE id = :id;
