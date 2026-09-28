-- name: LockWorkspacePlan :one
-- Taken for the whole of a create or fork, so two cannot both pass the quota.
SELECT
  plan
FROM
  app.workspace
WHERE
  id = $1
  AND deleted_at IS NULL
FOR UPDATE;
