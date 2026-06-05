-- name: DeleteWorkspaceToUserTracked :exec
DELETE FROM workspace_to_user WHERE id = :id;
