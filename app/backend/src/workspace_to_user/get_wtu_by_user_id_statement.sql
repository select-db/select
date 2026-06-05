-- name: GetWorkspaceToUserByUserId :one
SELECT 
    w.* 
FROM 
    workspace w
    LEFT JOIN workspace_to_user wtu ON wtu.workspace_id = w.id
WHERE 
    wtu.user_id = :user_id
ORDER BY 
    w.name ASC 
LIMIT 1;