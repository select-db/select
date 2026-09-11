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

// Folder is a workspace this machine has a folder for.
type Folder struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// ListFolders returns every workspace this user has a folder for, current one
// first. A remembered path that no longer names its workspace is forgotten
// rather than offered.
func (w *Workspace) ListFolders() ([]Folder, error) {
	ctx := context.Background()

	folders := []Folder{}

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		return folders, nil
	}

	rows, err := w.Queries.ListWorkspacesByUserID(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	for _, row := range rows {
		path := row.LocalPath.Or("")
		if !folderNamesWorkspace(path, row.ID) {
			if path != "" {
				_ = w.forgetFolder(ctx, row.ID)
			}
			continue
		}
		folder := Folder{Path: path, Name: row.Name}
		if row.Current.Valid && row.Current.Bool {
			folders = append([]Folder{folder}, folders...)
			continue
		}
		folders = append(folders, folder)
	}

	return folders, nil
}

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
	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		return FolderState{Status: NoFolder}, nil
	}

	ws, err := w.Queries.GetCurrentWorkspace(ctx, u.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FolderState{Status: NoFolder}, nil
		}
		return FolderState{}, fmt.Errorf("get current workspace: %w", err)
	}

	folder := ws.LocalPath.Or("")
	if !folderNamesWorkspace(folder, ws.ID) {
		_ = w.forgetFolder(ctx, ws.ID)
		return FolderState{Status: NoFolder}, nil
	}
	return w.OpenFolder(folder)
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
