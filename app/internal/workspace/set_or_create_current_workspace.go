package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"selectDb/internal/db/generated"
	"selectDb/internal/fs_provider"
	"selectDb/internal/graph"
	"selectDb/internal/sample"
)

type SetOrCreateCurrentWorkspaceParams struct {
	UserID string
}

func (w *Workspace) SetOrCreateCurrentWorkspace(params SetOrCreateCurrentWorkspaceParams) (generated.Workspace, error) {
	ctx := context.Background()

	// 1 - Try to get the current workspace
	workspace, err := w.Queries.GetCurrentWorkspace(ctx, params.UserID)
	if err == nil {
		if err := w.ensureWorkspaceFolder(workspace); err != nil {
			return generated.Workspace{}, err
		}
		return workspace, nil
	}
	if err != sql.ErrNoRows {
		return generated.Workspace{}, fmt.Errorf("get current workspace: %w", err)
	}

	// 2 - Try to find any workspace linked to this user
	workspace, err = w.Queries.GetWorkspaceToUserByUserId(ctx, params.UserID)
	if err == nil {

		if err := w.Queries.ClearCurrentWorkspaceToUser(ctx); err != nil {
			return generated.Workspace{}, err
		}

		if updateErr := w.Queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
			UserID:      params.UserID,
			WorkspaceID: workspace.ID,
		}); updateErr != nil {
			return generated.Workspace{}, fmt.Errorf("set current workspace: %w", updateErr)
		}

		return workspace, nil
	}
	if err != sql.ErrNoRows {
		return generated.Workspace{}, fmt.Errorf("find workspace for user: %w", err)
	}

	// No workspace found
	// sync has not run yet or failed.
	return generated.Workspace{}, sql.ErrNoRows
}

// Ensures the workspace root folder exists on disk
func (w *Workspace) EnsureWorkspaceFolderByID(workspaceID, _ string) error {
	return w.ensureWorkspaceFolder(generated.Workspace{ID: workspaceID, Name: ""})
}

// ensureWorkspaceFolder makes sure the workspace root folder exists on the
// user's filesystem, and seeds the sample workspace into it when it is empty.
//
// The seed is gated on emptiness rather than on having just created the
// directory, because a workspace root is a folder the user chose. Dropping
// cohorts.sql and a .lint into a repository somebody already has work in would
// be vandalism; an empty folder is the only one where a sample is a gift
// rather than a mess.
//
// Personal files (.theme, .config) live in the per-user config dir, not here.
func (w *Workspace) ensureWorkspaceFolder(workspace generated.Workspace) error {
	if err := w.FSProvider.Mkdir(fs_provider.MkdirParams{
		URI: w.FSProvider.WorkspaceURIPrefix() + workspace.ID,
	}); err != nil {
		return fmt.Errorf("ensure workspace root directory: %w", err)
	}

	root, err := graph.WorkspaceRootPath(workspace.ID)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}

	empty, err := isEmptyDir(root)
	if err != nil {
		return fmt.Errorf("read workspace root: %w", err)
	}
	if !empty {
		return nil
	}

	if err := sample.Write(workspace.ID); err != nil {
		return fmt.Errorf("seed sample workspace: %w", err)
	}

	return nil
}

// isEmptyDir reports whether dir holds no entries at all, hidden ones included.
// A directory carrying only a .git is not empty: it is a clone somebody made.
func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
