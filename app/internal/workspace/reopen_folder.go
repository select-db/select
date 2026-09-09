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
	if folder == "" {
		return LastFolder{}, nil
	}

	cfg, err := graph.ReadWorkspaceConfig(folder)
	if err != nil || cfg.WorkspaceID != ws.ID {
		// Gone, or now some other workspace's folder. Forget it quietly: the
		// user is looking at the no-folder screen either way, and an error
		// about a path they have not mentioned is noise.
		_ = w.forgetFolder(ctx, ws.ID)
		return LastFolder{}, nil
	}

	return LastFolder{Path: folder, Name: ws.Name}, nil
}

// ReopenLastFolder runs after login, so a returning user skips the picker.
func (w *Workspace) ReopenLastFolder() (OpenFolderResult, error) {
	last, err := w.GetLastFolder()
	if err != nil {
		return OpenFolderResult{}, err
	}
	if last.Path == "" {
		return OpenFolderResult{State: OpenFolderNone}, nil
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
