-- name: UpsertGroupForSync :exec
; -- @no-track
INSERT INTO "group" (id, workspace_id, name, source, external_id)
VALUES (:id, :workspace_id, :name, :source, :external_id)
ON CONFLICT (id) DO UPDATE SET
    workspace_id = excluded.workspace_id,
    name = excluded.name,
    source = excluded.source,
    external_id = excluded.external_id;
