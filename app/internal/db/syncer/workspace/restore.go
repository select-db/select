package workspace

import (
	"context"
	"database/sql"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

// Ensurer ensures a workspace folder exists on disk and can remove it. Optional; set by app wiring.
type Ensurer interface {
	EnsureWorkspaceFolderByID(workspaceID, name string) error
	RemoveWorkspaceFolderByID(workspaceID string) error
}

// Restore upserts the server-authoritative workspace row, and ensures the
// workspace folder exists on disk when the workspace is new. Materializing the
// git repository to match git_remote_url is handled separately by the syncer's
// reconcile pass, which is idempotent and self-healing.
func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any, ensurer Ensurer) error {
	id := utils.MapGetString(payload, "id")
	name := utils.MapGetString(payload, "name")
	if id == "" || name == "" {
		return nil
	}
	payloadGitRemote := utils.MapGetStringPtr(payload, "git_remote_url")

	existedBefore := true
	existing, err := queries.GetWorkspaceByID(ctx, id)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		existedBefore = false
	}

	gitRemote := db_types.JSONNullString{}
	if payloadGitRemote != nil {
		gitRemote = db_types.NewJSONNullString(*payloadGitRemote)
	}
	ownerID := db_types.JSONNullString{}
	if oid := utils.MapGetString(payload, "owner_id"); oid != "" {
		ownerID = db_types.NewJSONNullString(oid)
	}
	// Execution limits are server-authoritative once the backend manages them.
	// Until then a pulled row may omit these fields; in that case we keep the
	// values already stored locally (defaults for a brand-new workspace) instead
	// of resetting them.
	defaultTimeout := 30000
	defaultSize := 100
	if existedBefore {
		defaultTimeout = int(existing.StatementTimeoutMs)
		defaultSize = int(existing.MaxResultSizeMb)
	}
	statementTimeoutMs := utils.MapGetIntOr(payload, "statement_timeout_ms", defaultTimeout)
	maxResultSizeMB := utils.MapGetIntOr(payload, "max_result_size_mb", defaultSize)

	if err := queries.UpsertWorkspaceForSync(ctx, generated.UpsertWorkspaceForSyncParams{
		ID:                 id,
		Name:               name,
		GitRemoteUrl:       gitRemote,
		OwnerID:            ownerID,
		StatementTimeoutMs: int64(statementTimeoutMs),
		MaxResultSizeMb:    int64(maxResultSizeMB),
	}); err != nil {
		return err
	}

	if ensurer != nil && !existedBefore {
		if err := ensurer.EnsureWorkspaceFolderByID(id, name); err != nil {
			return err
		}
	}
	return nil
}
