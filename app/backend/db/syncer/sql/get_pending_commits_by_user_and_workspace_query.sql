-- name: GetPendingCommitsByUserAndWorkspace :many
SELECT
    id, created_at, operation, table_name, object_id, payload, user_id, workspace_id
FROM
    mutation_commit
WHERE
    user_id = :user_id
    AND workspace_id = :workspace_id
ORDER BY created_at ASC
LIMIT :limit;
