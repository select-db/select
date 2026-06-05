-- name: GetCurrentUser :one
SELECT 
    u.* 
FROM 
    user u
    LEFT JOIN workspace_to_user wtu ON wtu.user_id = u.id
WHERE 
    wtu.current = 1
LIMIT 1;