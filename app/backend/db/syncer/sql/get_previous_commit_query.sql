-- name: GetPreviousCommit :one
SELECT 
  id,
  payload
FROM 
  mutation_commit
WHERE 
  operation = :operation
  AND table_name = :table_name
  AND object_id = :object_id
  AND user_id = :user_id
  AND workspace_id = :workspace_id;