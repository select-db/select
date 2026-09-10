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

// OpenFolderState is which screen the frontend owes the user next.
type OpenFolderState string

const (
	// Signed in, nothing open.
	OpenFolderNone OpenFolderState = "no_folder"

	// The workspace is set up and the graph is built.
	OpenFolderOpened OpenFolderState = "opened"

	// No config, or one naming a workspace this server does not have.
	OpenFolderNeedsInit OpenFolderState = "needs_init"

	// The workspace lives on another server, whose permissions gate every query.
	OpenFolderWrongServer OpenFolderState = "wrong_server"
)

type OpenFolderResult struct {
	State OpenFolderState `json:"state"`
	Path  string          `json:"path"`

	// The folder's own name, offered as the workspace name on the init screen.
	SuggestedName string `json:"suggestedName,omitempty"`

	// Set when the config names a workspace this server does not have, so the
	// init screen can say so rather than pretend the folder was never one.
	StaleWorkspaceID string `json:"staleWorkspaceId,omitempty"`

	FolderServer  string `json:"folderServer,omitempty"`
	CurrentServer string `json:"currentServer,omitempty"`
}

// PickFolder returns "" when the user cancels.
func (w *Workspace) PickFolder() (string, error) {
	return desktop.OpenDirectory("Open Folder")
}

// OpenFolder is the only thing that sets up a workspace: not login, not sync.
func (w *Workspace) OpenFolder(path string) (OpenFolderResult, error) {
	folder, err := normalizeFolder(path)
	if err != nil {
		return OpenFolderResult{}, err
	}

	result := OpenFolderResult{Path: folder, SuggestedName: filepath.Base(folder)}

	currentServer, err := server.ReadCurrentDomain()
	if err != nil {
		return OpenFolderResult{}, fmt.Errorf("read current server: %w", err)
	}
	result.CurrentServer = currentServer

	cfg, err := graph.ReadWorkspaceConfig(folder)
	if err != nil {
		if errors.Is(err, graph.ErrNoWorkspaceConfig) {
			result.State = OpenFolderNeedsInit
			return result, nil
		}
		// Unusable config: reporting it beats overwriting the workspace it named.
		return OpenFolderResult{}, err
	}

	if cfg.Server != currentServer {
		result.State = OpenFolderWrongServer
		result.FolderServer = cfg.Server
		return result, nil
	}

	known, err := w.workspaceExists(cfg.WorkspaceID)
	if err != nil {
		return OpenFolderResult{}, err
	}
	if !known {
		result.State = OpenFolderNeedsInit
		result.StaleWorkspaceID = cfg.WorkspaceID
		return result, nil
	}

	if err := w.adoptFolder(cfg.WorkspaceID, folder); err != nil {
		return OpenFolderResult{}, err
	}

	result.State = OpenFolderOpened
	return result, nil
}

// workspaceExists pulls before answering: a teammate who just cloned has the
// folder before the local database has heard of the workspace. "Not a member"
// and "no such workspace" both come back false, since both need an init.
func (w *Workspace) workspaceExists(workspaceID string) (bool, error) {
	ctx := context.Background()

	if _, err := w.Queries.GetWorkspaceByID(ctx, workspaceID); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("look up workspace: %w", err)
	}

	if w.PullFunc == nil {
		return false, nil
	}
	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		return false, nil
	}
	if err := w.PullFunc(ctx, u.ID); err != nil {
		return false, fmt.Errorf("pull workspaces: %w", err)
	}

	_, err = w.Queries.GetWorkspaceByID(ctx, workspaceID)
	return err == nil, nil
}

// adoptFolder makes workspaceID current, rooted at folder.
func (w *Workspace) adoptFolder(workspaceID, folder string) error {
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

	graph.SetOpenWorkspaceRoot(workspaceID, folder)

	if err := w.seedIfEmpty(workspaceID); err != nil {
		return err
	}

	return w.reloadGraph()
}

// CloseFolder leaves the user signed in on the no-folder screen. The folder on
// disk is untouched.
func (w *Workspace) CloseFolder() error {
	graph.ClearOpenWorkspaceRoot()
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
