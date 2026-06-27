-- name: ListHistory :many
SELECT id, statement, affected_rows, row_count, duration_ms, errors, uri, dsn, created_at, workspace_id, db_instance_id
FROM history
WHERE workspace_id = :workspace_id
  AND created_at >= datetime('now', '-7 days')
ORDER BY created_at DESC, id DESC
LIMIT :limit OFFSET :offset;
