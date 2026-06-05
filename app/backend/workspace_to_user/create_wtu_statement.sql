-- name: CreateWorkspaceToUser :one
; -- @no-track
INSERT INTO workspace_to_user (
    id, 
    workspace_id, 
    user_id
) VALUES (
    :id, 
    :workspace_id, 
    :user_id
)
ON CONFLICT (user_id, workspace_id) DO NOTHING
RETURNING *;