package workspace

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/db/generated"
)

// SwitchWorkspace sets the current workspace for the current user, ensures its folder exists,
// rebuilds the workspace graph, and emits workspaceGraphUpdated when ReloadHooks is set.
func (w *Workspace) SwitchWorkspace(workspaceID string) error {
	ctx := context.Background()
	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no current user")
		}
		return fmt.Errorf("get current user: %w", err)
	}
	if err := w.Queries.ClearCurrentWorkspaceToUser(ctx); err != nil {
		return fmt.Errorf("clear current workspace: %w", err)
	}
	if err := w.Queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      u.ID,
		WorkspaceID: workspaceID,
	}); err != nil {
		return fmt.Errorf("set current workspace: %w", err)
	}
	if err := w.EnsureWorkspaceFolderByID(workspaceID, ""); err != nil {
		return fmt.Errorf("ensure workspace folder: %w", err)
	}
	if h := w.ReloadHooks; h != nil {
		if h.ReconcileGitRemote != nil {
			h.ReconcileGitRemote(workspaceID)
		}
		if h.BuildWorkspaceGraph != nil {
			if err := h.BuildWorkspaceGraph(); err != nil {
				return fmt.Errorf("build workspace graph: %w", err)
			}
		}
		if h.EmitWorkspaceGraphUpdated != nil {
			h.EmitWorkspaceGraphUpdated()
		}
	}
	return nil
}
