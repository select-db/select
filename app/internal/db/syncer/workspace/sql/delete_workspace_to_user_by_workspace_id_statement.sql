-- name: DeleteWorkspaceToUserByWorkspaceID :exec
; -- @no-track
DELETE FROM workspace_to_user
WHERE workspace_id = :workspace_id;
