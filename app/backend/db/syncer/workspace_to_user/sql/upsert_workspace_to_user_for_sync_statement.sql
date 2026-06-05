-- name: UpsertWorkspaceToUserForSync :exec
; -- @no-track
INSERT INTO workspace_to_user (id, workspace_id, user_id, current)
VALUES (:id, :workspace_id, :user_id, FALSE)
ON CONFLICT (user_id, workspace_id) DO UPDATE SET
    id = excluded.id,
    workspace_id = excluded.workspace_id,
    user_id = excluded.user_id;
