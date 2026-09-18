-- name: DeleteHistoryOlderThan7Days :exec
; -- @no-track
DELETE FROM history
WHERE created_at < datetime('now', '-7 days');

-- name: PruneHistoryForWorkspace :exec
; -- @no-track
DELETE FROM history
WHERE history.workspace_id = :workspace_id
  AND history.id NOT IN (
    SELECT h.id FROM history h
    WHERE h.workspace_id = :workspace_id
    ORDER BY h.created_at DESC, h.id DESC
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
