-- name: ClearCurrentWorkspaceToUser :exec
; -- @no-track
UPDATE 
    workspace_to_user 
SET 
    current = FALSE 
WHERE 
    current = TRUE;