-- name: UpdateWorkspaceExecutionLimits :exec
UPDATE workspace
SET statement_timeout_ms = :statement_timeout_ms,
    max_result_size_mb = :max_result_size_mb
WHERE id = :id;
