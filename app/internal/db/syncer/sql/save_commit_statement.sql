-- name: SaveCommit :one
; -- @no-track
INSERT INTO mutation_commit (
    id,
    operation,
    table_name,
    object_id,
    payload,
    user_id,
    workspace_id
) VALUES (
    :id, 
    :operation, 
    :table_name, 
    :object_id, 
    :payload,
    :user_id,
    :workspace_id
)
ON CONFLICT(operation, table_name, object_id, user_id, workspace_id) 
DO NOTHING
RETURNING *;