-- name: UpsertUserToGroupForSync :exec
; -- @no-track
INSERT INTO user_to_group (id, user_id, group_id, workspace_id, source)
VALUES (:id, :user_id, :group_id, :workspace_id, :source)
ON CONFLICT (id) DO UPDATE SET
    user_id = excluded.user_id,
    group_id = excluded.group_id,
    workspace_id = excluded.workspace_id,
    source = excluded.source;
