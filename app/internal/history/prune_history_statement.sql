-- name: DeleteHistoryOlderThan7Days :exec
; -- @no-track
DELETE FROM history
WHERE created_at < datetime('now', '-7 days');

-- name: PruneHistoryForWorkspace :exec
; -- @no-track
DELETE FROM history
WHERE workspace_id = :workspace_id
  AND id NOT IN (
    SELECT id FROM history
    WHERE workspace_id = :workspace_id
    ORDER BY created_at DESC, id DESC
    LIMIT 100
  );

-- name: PruneHistoryAllWorkspaces :exec
; -- @no-track
DELETE FROM history
WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (
            PARTITION BY workspace_id
            ORDER BY created_at DESC, id DESC
        ) AS rn
        FROM history
    ) WHERE rn > 100
);
