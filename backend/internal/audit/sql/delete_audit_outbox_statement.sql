-- name: DeleteAuditOutbox :exec
DELETE FROM audit.outbox WHERE id = ANY($1::bigint[]);
