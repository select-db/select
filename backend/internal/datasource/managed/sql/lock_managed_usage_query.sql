-- name: LockManagedUsage :one
-- Locks the workspace row for the whole of a create or fork, so two cannot both pass the quota.
SELECT
  w.plan,
  (
    SELECT count(*)
    FROM app.datasource d
    WHERE d.workspace_id = w.id AND d.cellar_id IS NOT NULL AND d.state IS DISTINCT FROM 'deleting'
  ) AS database_count,
  (
    SELECT COALESCE(sum(d.size_bytes), 0)
    FROM app.datasource d
    WHERE d.workspace_id = w.id AND d.cellar_id IS NOT NULL AND d.state IS DISTINCT FROM 'deleting'
  )::bigint AS total_bytes
FROM
  app.workspace w
WHERE
  w.id = $1
  AND w.deleted_at IS NULL
FOR UPDATE OF w;
