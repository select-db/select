package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/graph"
)

// LastFolder is what the no-folder screen offers to reopen.
type LastFolder struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// GetLastFolder returns the folder the current user last had open, when it is
// still there and still names the same workspace.
func (w *Workspace) GetLastFolder() (LastFolder, error) {
	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		return LastFolder{}, nil
	}

	ws, err := w.Queries.GetCurrentWorkspace(ctx, u.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LastFolder{}, nil
		}
		return LastFolder{}, fmt.Errorf("get current workspace: %w", err)
	}

	folder := ws.LocalPath.Or("")
	if !folderNamesWorkspace(folder, ws.ID) {
		// Gone, or now another workspace's. Forget it quietly.
		_ = w.forgetFolder(ctx, ws.ID)
		return LastFolder{}, nil
	}
	return LastFolder{Path: folder, Name: ws.Name}, nil
}

// folderNamesWorkspace reports whether path still holds a config naming
// workspaceID.
//
// local_path is a hint, never an authority: a folder re-inited as a different
// workspace leaves the old row pointing at it, so acting on the memory without
// asking the folder acts on somebody else's.
func folderNamesWorkspace(path, workspaceID string) bool {
	if path == "" {
		return false
	}
	cfg, err := graph.ReadWorkspaceConfig(path)
	return err == nil && cfg.WorkspaceID == workspaceID
}

// ReopenLastFolder runs after login, so a returning user skips the picker.
func (w *Workspace) ReopenLastFolder() (FolderState, error) {
	last, err := w.GetLastFolder()
	if err != nil {
		return FolderState{}, err
	}
	if last.Path == "" {
		return FolderState{Status: NoFolder}, nil
	}
	return w.OpenFolder(last.Path)
}

func (w *Workspace) rememberFolder(ctx context.Context, workspaceID, folder string) error {
	if err := w.Queries.UpdateWorkspaceLocalPath(ctx, generated.UpdateWorkspaceLocalPathParams{
		ID:        workspaceID,
		LocalPath: db_types.NewJSONNullString(folder),
	}); err != nil {
		return fmt.Errorf("remember workspace folder: %w", err)
	}
	return nil
}

func (w *Workspace) forgetFolder(ctx context.Context, workspaceID string) error {
	return w.Queries.UpdateWorkspaceLocalPath(ctx, generated.UpdateWorkspaceLocalPathParams{
		ID:        workspaceID,
		LocalPath: db_types.JSONNullString{},
	})
}
