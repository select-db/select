-- name: GetManagedUsage :one
SELECT
  count(*) AS database_count,
  COALESCE(sum(size_bytes), 0)::bigint AS total_bytes
FROM
  app.datasource
WHERE
  workspace_id = $1
  AND cellar_id IS NOT NULL
  AND state <> 'deleting';
