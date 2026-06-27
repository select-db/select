-- name: CreateHistory :one
; -- @no-track
INSERT INTO history (
    id,
    dsn,
    uri,
    statement,
    affected_rows,
    row_count,
    duration_ms,
    errors,
    workspace_id,
    db_instance_id,
    created_at
) VALUES (
    :id,
    :dsn,
    :uri,
    :statement,
    :affected_rows,
    :row_count,
    :duration_ms,
    :errors,
    :workspace_id,
    :db_instance_id,
    CURRENT_TIMESTAMP
)
RETURNING id, statement, affected_rows, row_count, duration_ms, errors, uri, dsn, created_at, workspace_id, db_instance_id;
