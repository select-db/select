-- name: GetWorkspaceToUserByUserAndWorkspace :one
SELECT
  id,
  workspace_id,
  user_id
FROM
  workspace_to_user
WHERE
  user_id =:user_id
  AND workspace_id =:workspace_id;