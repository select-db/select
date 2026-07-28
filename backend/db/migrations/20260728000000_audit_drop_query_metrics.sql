-- +goose Up
-- +goose StatementBegin
-- Drop the two query-only metric columns from the audit log. They were
-- speculative and never a stable part of the event contract; per-query metrics
-- (execution duration, returned row count) will come back as dedicated columns
-- only if users ask for them. Dropping on the partitioned parent cascades to
-- every partition and removes the columns' comments along with them.
ALTER TABLE audit.event
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS returned_row_count;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit.event
    ADD COLUMN duration_ms        BIGINT,
    ADD COLUMN returned_row_count BIGINT;
-- +goose StatementEnd
