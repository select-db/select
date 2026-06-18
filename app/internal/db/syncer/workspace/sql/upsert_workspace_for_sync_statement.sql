-- name: UpsertWorkspaceForSync :exec
; -- @no-track
INSERT INTO workspace (id, name, git_remote_url, owner_id)
VALUES (:id, :name, :git_remote_url, :owner_id)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    git_remote_url = excluded.git_remote_url,
    owner_id = excluded.owner_id;
