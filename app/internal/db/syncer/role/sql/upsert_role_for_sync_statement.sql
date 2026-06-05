-- name: UpsertRoleForSync :exec
; -- @no-track
INSERT INTO role (id, workspace_id, name)
VALUES (:id, :workspace_id, :name)
ON CONFLICT (id) DO UPDATE SET
    workspace_id = excluded.workspace_id,
    name = excluded.name;
