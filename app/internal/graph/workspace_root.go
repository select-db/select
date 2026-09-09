package graph

import (
	"fmt"
	"path/filepath"
	"sync"

	"selectDb/internal/server"
)

// The open workspace and its folder. One window means one open workspace, so
// this is a pair rather than a map, and an id that is not the open one is an
// error rather than a plausible path.
var (
	openMu          sync.RWMutex
	openWorkspaceID string
	openRoot        string
)

// SetOpenWorkspaceRoot is called by the open-folder flow before the graph builds.
func SetOpenWorkspaceRoot(workspaceID, root string) {
	openMu.Lock()
	defer openMu.Unlock()
	openWorkspaceID = workspaceID
	openRoot = root
}

// ClearOpenWorkspaceRoot forgets the open workspace.
func ClearOpenWorkspaceRoot() {
	openMu.Lock()
	defer openMu.Unlock()
	openWorkspaceID = ""
	openRoot = ""
}

// OpenWorkspaceRoot returns false when no folder is open.
func OpenWorkspaceRoot() (workspaceID, root string, ok bool) {
	openMu.RLock()
	defer openMu.RUnlock()
	if openWorkspaceID == "" || openRoot == "" {
		return "", "", false
	}
	return openWorkspaceID, openRoot, true
}

// WorkspaceRootPath returns the absolute filesystem path to the workspace root
// directory for the given workspace ID.
//
// Until the whole app opens folders rather than deriving them, an id that is
// not the open one falls back to the managed layout this is replacing, so the
// callers still on that path keep working. That fallback goes away with the
// managed layout itself.
func WorkspaceRootPath(workspaceID string) (string, error) {
	if id, root, ok := OpenWorkspaceRoot(); ok && id == workspaceID {
		return root, nil
	}
	return managedWorkspaceRootPath(workspaceID)
}

// managedWorkspaceRootPath is the old arithmetic: <server folder>/workspaces/<id>.
func managedWorkspaceRootPath(workspaceID string) (string, error) {
	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		return "", err
	}
	if serverRoot == "" {
		return "", fmt.Errorf("no current server")
	}
	return filepath.Join(serverRoot, "workspaces", workspaceID), nil
}
