package workspace

import (
	"context"
	"database/sql"

	"selectDb/backend/db/db_types"
	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
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
	_, err := queries.GetWorkspaceByID(ctx, id)
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
	if err := queries.UpsertWorkspaceForSync(ctx, generated.UpsertWorkspaceForSyncParams{
		ID:           id,
		Name:         name,
		GitRemoteUrl: gitRemote,
		OwnerID:      ownerID,
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
