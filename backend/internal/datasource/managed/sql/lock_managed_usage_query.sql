-- name: LockManagedWorkspace :one
-- Locks the workspace row for the whole of a create or fork, so two cannot both pass the quota.
-- ManagedUsage counts in a statement of its own: one that waited for this lock
-- still reads the snapshot it started on.
SELECT
  w.plan
FROM
  app.workspace w
WHERE
  w.id = $1
  AND w.deleted_at IS NULL
FOR UPDATE OF w;

-- name: ManagedUsage :one
SELECT
  count(*) AS database_count,
  COALESCE(sum(d.size_bytes), 0)::bigint AS total_bytes
FROM
  app.datasource d
WHERE
  d.workspace_id = $1
  AND d.state IS NOT NULL
  AND d.state IS DISTINCT FROM 'deleting';
