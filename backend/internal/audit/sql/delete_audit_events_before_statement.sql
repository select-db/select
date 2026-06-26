-- name: DeleteAuditEventsBefore :execrows
DELETE FROM audit.event WHERE occurred_at < $1;
