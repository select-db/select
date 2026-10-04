-- name: GetManagedDatasourceToReconcile :one
-- What the reconciler needs to decide on a database the cellar holds. The workspace
-- is joined without its deleted_at filter: a deleted workspace is a reason to purge.
SELECT
  d.cellar_id,
  d.state,
  (w.deleted_at IS NOT NULL)::boolean AS workspace_deleted
FROM
  app.datasource d
  JOIN app.workspace w ON w.id = d.workspace_id
WHERE
  d.id = $1
  AND d.cellar_id IS NOT NULL;

-- name: ListDeletingManagedDatasources :many
SELECT d.id FROM app.datasource d WHERE d.cellar_id = $1 AND d.state = 'deleting';

-- name: DeleteManagedDatasourceRow :exec
-- The row of a database the reconciler purged, or that the cellar never held.
DELETE FROM app.datasource WHERE id = $1 AND cellar_id = $2;

-- name: SetManagedDatasourceSize :exec
-- The size the cellar reports now. The row is written once, at creation, and the
-- workspace's quota adds these up, so without this a database that grew counts
-- as the empty file it was.
UPDATE app.datasource
SET size_bytes = sqlc.arg(size_bytes)
WHERE id = sqlc.arg(id)
  AND cellar_id = sqlc.arg(cellar_id)
  AND state IS DISTINCT FROM 'deleting'
  AND size_bytes IS DISTINCT FROM sqlc.arg(size_bytes);
