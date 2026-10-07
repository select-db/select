-- name: DeleteDatasource :exec
-- A managed database is only marked: its file stays until the reconciler purges it.
WITH marked AS (
  UPDATE app.datasource managed
  SET state = 'deleting', updated_at = now()
  WHERE managed.id = sqlc.arg(id) AND managed.workspace_id = sqlc.arg(workspace_id) AND managed.state IS NOT NULL
  RETURNING managed.id
)
DELETE FROM app.datasource unmanaged
WHERE
  unmanaged.id = sqlc.arg(id)
  AND unmanaged.workspace_id = sqlc.arg(workspace_id)
  AND unmanaged.state IS NULL;
