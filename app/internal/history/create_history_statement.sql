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
    errors
) VALUES (
    :id,
    :dsn,
    :uri,
    :statement,
    :affected_rows,
    :row_count,
    :duration_ms,
    :errors
)
RETURNING *;