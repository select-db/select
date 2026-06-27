-- name: GetAuditOutboxBatch :many
SELECT id, event_json
FROM audit.outbox
ORDER BY id
FOR UPDATE SKIP LOCKED
LIMIT $1;
