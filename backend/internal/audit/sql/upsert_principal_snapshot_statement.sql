-- name: UpsertPrincipalSnapshot :exec
INSERT INTO audit.principal_snapshot (snapshot_hash, workspace_id, snapshot)
VALUES ($1, $2, $3)
ON CONFLICT (snapshot_hash) DO NOTHING;
