-- name: UpsertUserToRoleForSync :exec
; -- @no-track
INSERT INTO user_to_role (id, user_id, role_id, workspace_id)
VALUES (:id, :user_id, :role_id, :workspace_id)
ON CONFLICT (id) DO UPDATE SET
    user_id = excluded.user_id,
    role_id = excluded.role_id,
    workspace_id = excluded.workspace_id;
