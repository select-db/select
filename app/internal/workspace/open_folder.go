package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"selectDb/internal/db/generated"
	"selectDb/internal/desktop"
	"selectDb/internal/graph"
	"selectDb/internal/server"
)

// WorkspaceStatus is what opening a folder found, and so which screen the
// frontend owes the user next.
type WorkspaceStatus string

const (
	// Signed in, nothing open.
	NoFolder WorkspaceStatus = "no_folder"

	// The workspace is set up and the graph is built.
	Ready WorkspaceStatus = "ready"

	// The folder holds no config, so there is no workspace to open yet.
	NeedsSetup WorkspaceStatus = "needs_setup"

	// The config names a workspace this user cannot open: deleted, revoked, or
	// never theirs.
	NoAccess WorkspaceStatus = "no_access"

	// The workspace lives on another server, whose permissions gate every query.
	WrongServer WorkspaceStatus = "wrong_server"
)

// FolderState is the status plus whatever its screen needs to say something
// specific.
type FolderState struct {
	Status WorkspaceStatus `json:"status"`
	Path   string          `json:"path"`

	// The folder's own name, offered as the workspace name on the setup screen.
	SuggestedName string `json:"suggestedName,omitempty"`

	// The workspace the config names, empty when the folder has none.
	WorkspaceID string `json:"workspaceId,omitempty"`

	FolderServer  string `json:"folderServer,omitempty"`
	CurrentServer string `json:"currentServer,omitempty"`
}

// PickFolder returns "" when the user cancels.
func (w *Workspace) PickFolder() (string, error) {
	return desktop.OpenDirectory("Open Folder")
}

// OpenFolder is the only thing that sets up a workspace: not login, not sync.
func (w *Workspace) OpenFolder(path string) (FolderState, error) {
	folder, err := resolveFolder(path)
	if err != nil {
		return FolderState{}, err
	}

	result := FolderState{Path: folder, SuggestedName: filepath.Base(folder)}

	currentServer, err := server.ReadCurrentDomain()
	if err != nil {
		return FolderState{}, fmt.Errorf("read current server: %w", err)
	}
	result.CurrentServer = currentServer

	cfg, err := graph.ReadWorkspaceConfig(folder)
	if err != nil {
		if errors.Is(err, graph.ErrNoWorkspaceConfig) {
			result.Status = NeedsSetup
			return result, nil
		}
		// Unusable config: reporting it beats overwriting the workspace it named.
		return FolderState{}, err
	}
	result.WorkspaceID = cfg.WorkspaceID

	if cfg.Server != currentServer {
		result.Status = WrongServer
		result.FolderServer = cfg.Server
		return result, nil
	}

	member, err := w.isMember(cfg.WorkspaceID)
	if err != nil {
		return FolderState{}, err
	}
	if !member {
		result.Status = NoAccess
		return result, nil
	}

	if err := w.setCurrentWorkspace(cfg.WorkspaceID, folder); err != nil {
		return FolderState{}, err
	}

	result.Status = Ready
	return result, nil
}

// isMember answers on the membership row rather than on the workspace row: a
// revoked membership deletes the first and leaves the second, so asking whether
// the workspace exists opens a folder every query then refuses.
//
// It pulls before saying no, since a teammate who just cloned has the folder
// before the local database has heard of the workspace.
func (w *Workspace) isMember(workspaceID string) (bool, error) {
	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("no current user")
		}
		return false, fmt.Errorf("get current user: %w", err)
	}

	member, err := w.hasMemberRow(ctx, u.ID, workspaceID)
	if err != nil || member {
		return member, err
	}

	if w.PullFunc == nil {
		return false, nil
	}
	if err := w.PullFunc(ctx, u.ID); err != nil {
		return false, fmt.Errorf("pull workspaces: %w", err)
	}
	return w.hasMemberRow(ctx, u.ID, workspaceID)
}

func (w *Workspace) hasMemberRow(ctx context.Context, userID, workspaceID string) (bool, error) {
	_, err := w.Queries.GetWorkspaceToUserByUserAndWorkspace(ctx, generated.GetWorkspaceToUserByUserAndWorkspaceParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("look up workspace membership: %w", err)
}

// setCurrentWorkspace makes workspaceID the current workspace, rooted at folder.
func (w *Workspace) setCurrentWorkspace(workspaceID, folder string) error {
	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	if err := w.rememberFolder(ctx, workspaceID, folder); err != nil {
		return err
	}

	graph.SetOpenWorkspace(workspaceID, folder)

	if err := w.seedSampleIfEmpty(workspaceID); err != nil {
		return err
	}

	return w.reloadGraph()
}

// CloseFolder leaves the user signed in on the no-folder screen. The folder on
// disk is untouched.
func (w *Workspace) CloseFolder() error {
	graph.ClearOpenWorkspace()
	if w.Graph != nil {
		w.Graph.InvalidateWorkspaceGraph()
	}
	if h := w.ReloadHooks; h != nil && h.EmitWorkspaceClosed != nil {
		h.EmitWorkspaceClosed()
	}
	return nil
}

func (w *Workspace) reloadGraph() error {
	h := w.ReloadHooks
	if h == nil {
		return nil
	}
	if h.BuildWorkspaceGraph != nil {
		if err := h.BuildWorkspaceGraph(); err != nil {
			return fmt.Errorf("build workspace graph: %w", err)
		}
	}
	if h.EmitWorkspaceGraphUpdated != nil {
		h.EmitWorkspaceGraphUpdated()
	}
	return nil
}
