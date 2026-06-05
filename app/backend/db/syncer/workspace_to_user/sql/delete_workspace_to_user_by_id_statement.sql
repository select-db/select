-- name: DeleteWorkspaceToUserByID :exec
; -- @no-track
DELETE FROM workspace_to_user
WHERE id = :id AND workspace_id = :workspace_id;
