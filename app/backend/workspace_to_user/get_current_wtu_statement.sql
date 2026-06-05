-- name: GetCurrentWorkspaceToUser :one
SELECT 
    * 
FROM 
    workspace_to_user wtu
WHERE 
    wtu.current = true
LIMIT 1;