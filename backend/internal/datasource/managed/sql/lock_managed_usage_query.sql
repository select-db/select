-- name: LockManagedWorkspace :one
-- Locks the workspace row for the whole of a create or fork, so two cannot both pass the quota.
-- The usage is read by ManagedUsage, a statement of its own: in READ COMMITTED a
-- statement that waited for this lock still counts with the snapshot it started
-- on, so a count read here would miss the create that held the lock before.
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
  AND d.cellar_id IS NOT NULL
  AND d.state IS DISTINCT FROM 'deleting';
