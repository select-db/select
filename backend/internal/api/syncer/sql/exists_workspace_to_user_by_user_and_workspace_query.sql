-- name: ExistsWorkspaceToUserByUserAndWorkspace :one
SELECT EXISTS (
    SELECT 1 
    FROM app.workspace_to_user 
    WHERE 
        user_id = $1 AND workspace_id = $2
);
