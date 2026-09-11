package workspace

import (
	"context"
	"fmt"
	"strings"

	"path/filepath"

	"selectDb/internal/api"
	"selectDb/internal/graph"
	"selectDb/internal/server"
)

type createWorkspaceResponse struct {
	ID                string `json:"id"`
	WorkspaceToUserID string `json:"workspace_to_user_id"`
	Name              string `json:"name"`
	OwnerID           string `json:"owner_id"`
}

// CreateWorkspaceInFolder creates the workspace on the server, writes the config
// that names it, and opens it. An empty name defaults to the folder's.
func (w *Workspace) CreateWorkspaceInFolder(path, name string) (FolderState, error) {
	folder, err := resolveFolder(path)
	if err != nil {
		return FolderState{}, err
	}

	ctx := context.Background()

	userID, err := w.currentUserID(ctx)
	if err != nil {
		return FolderState{}, err
	}

	currentServer, err := server.ReadCurrentDomain()
	if err != nil {
		return FolderState{}, fmt.Errorf("read current server: %w", err)
	}
	if currentServer == "" {
		return FolderState{}, fmt.Errorf("no current server")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(folder)
	}

	var resp createWorkspaceResponse
	if err := api.Fetch(ctx, "POST", "workspaces", map[string]string{"name": name}, nil, &resp); err != nil {
		return FolderState{}, fmt.Errorf("create workspace on server: %w", err)
	}

	ws, err := w.insertWorkspace(newWorkspaceParams{
		ID:                resp.ID,
		WorkspaceToUserID: resp.WorkspaceToUserID,
		UserID:            userID,
		Name:              resp.Name,
	})
	if err != nil {
		return FolderState{}, fmt.Errorf("create workspace locally: %w", err)
	}

	// Written last, because this file is what makes the folder a workspace: a
	// failed create must leave the folder as it was.
	if err := graph.WriteWorkspaceConfig(folder, currentServer, ws.ID); err != nil {
		return FolderState{}, err
	}

	// Opened rather than reported open, so Ready has one definition.
	return w.OpenFolder(folder)
}
