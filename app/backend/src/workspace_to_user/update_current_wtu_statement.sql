-- name: UpdateCurrentWorkspaceToUser :exec
; -- @no-track
UPDATE workspace_to_user
SET current = TRUE
WHERE id = (
    SELECT 
        wtu.id
    FROM 
        workspace_to_user AS wtu
        LEFT JOIN workspace w ON w.id = wtu.workspace_id
    WHERE 
        wtu.user_id = :user_id
        AND wtu.workspace_id = :workspace_id
);