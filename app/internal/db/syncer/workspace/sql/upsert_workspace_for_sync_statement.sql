-- name: UpsertWorkspaceForSync :exec
; -- @no-track
INSERT INTO workspace (id, name, git_remote_url, owner_id, statement_timeout_ms, max_result_size_mb)
VALUES (:id, :name, :git_remote_url, :owner_id, :statement_timeout_ms, :max_result_size_mb)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    git_remote_url = excluded.git_remote_url,
    owner_id = excluded.owner_id,
    statement_timeout_ms = excluded.statement_timeout_ms,
    max_result_size_mb = excluded.max_result_size_mb;
