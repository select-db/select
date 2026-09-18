-- name: InsertAuditOutbox :exec
INSERT INTO audit.outbox (event_json) VALUES ($1);
