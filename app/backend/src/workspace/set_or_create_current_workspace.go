package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"selectDb/backend/db/generated"
	"selectDb/backend/graph"
	"selectDb/backend/src/fs_provider"
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
// user's filesystem at:
//
//	APP_ROOT/workspaces/<workspace.ID>
//
// where APP_ROOT is the directory returned by GetAppDataDir().
// When the directory is created for the first time, default files (.config,
// .theme, …) are seeded from the embedded defaults.
func (w *Workspace) ensureWorkspaceFolder(workspace generated.Workspace) error {
	root, err := graph.WorkspaceRootPath(workspace.ID)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}

	_, statErr := os.Stat(root)
	isNew := os.IsNotExist(statErr)

	placeholderURI := "selectdb://workspaces/" + workspace.ID + "/.selectdb_workspace"

	if err := w.FSProvider.Write(fs_provider.WriteParams{
		URI:     placeholderURI,
		Content: "",
	}); err != nil {
		return fmt.Errorf("ensure workspace root directory: %w", err)
	}

	if err := w.FSProvider.Delete(fs_provider.DeleteParams{
		URI:       placeholderURI,
		Recursive: false,
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cleanup workspace placeholder: %w", err)
	}

	if isNew {
		if err := graph.SeedDefaultFiles(root); err != nil {
			return fmt.Errorf("seed default files: %w", err)
		}
	}

	return nil
}
