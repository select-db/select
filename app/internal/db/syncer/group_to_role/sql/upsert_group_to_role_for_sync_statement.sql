-- name: UpsertGroupToRoleForSync :exec
; -- @no-track
INSERT INTO group_to_role (id, group_id, role_id, workspace_id)
VALUES (:id, :group_id, :role_id, :workspace_id)
ON CONFLICT (id) DO UPDATE SET
    group_id = excluded.group_id,
    role_id = excluded.role_id,
    workspace_id = excluded.workspace_id;
