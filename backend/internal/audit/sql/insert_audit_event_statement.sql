-- name: InsertAuditEvent :exec
INSERT INTO audit.event (
    workspace_id,
    occurred_at,
    domain,
    action,
    principal_hash,
    principal_id,
    principal_type,
    target_type,
    target_id,
    target_label,
    status,
    payload,
    duration_ms,
    returned_row_count,
    client_ip
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
);
