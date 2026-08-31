-- name: UpsertWorkspaceForSync :exec
; -- @no-track
INSERT INTO workspace (id, name, git_remote_url, owner_id, statement_timeout_ms, max_result_size_mb, logo)
VALUES (:id, :name, :git_remote_url, :owner_id, :statement_timeout_ms, :max_result_size_mb, :logo)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    git_remote_url = excluded.git_remote_url,
    owner_id = excluded.owner_id,
    statement_timeout_ms = excluded.statement_timeout_ms,
    max_result_size_mb = excluded.max_result_size_mb,
    logo = excluded.logo;
