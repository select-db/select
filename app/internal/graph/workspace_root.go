package graph

import (
	"fmt"
	"sync"
)

// The open workspace and its folder. One window means one open workspace, so
// this is a pair rather than a map, and an id that is not the open one is an
// error rather than a plausible path.
var (
	openMu          sync.RWMutex
	openWorkspaceID string
	openRoot        string
)

// SetOpenWorkspace is called by the open-folder flow before the graph builds.
func SetOpenWorkspace(workspaceID, root string) {
	openMu.Lock()
	defer openMu.Unlock()
	openWorkspaceID = workspaceID
	openRoot = root
}

// ClearOpenWorkspace forgets the open workspace.
func ClearOpenWorkspace() {
	openMu.Lock()
	defer openMu.Unlock()
	openWorkspaceID = ""
	openRoot = ""
}

// OpenWorkspace returns false when no folder is open.
func OpenWorkspace() (workspaceID, root string, ok bool) {
	openMu.RLock()
	defer openMu.RUnlock()
	if openWorkspaceID == "" || openRoot == "" {
		return "", "", false
	}
	return openWorkspaceID, openRoot, true
}

// WorkspaceRootPath is the one resolver every package goes through, so nothing
// can disagree about which directory an id names.
func WorkspaceRootPath(workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("no workspace folder is open")
	}
	id, root, ok := OpenWorkspace()
	if !ok {
		return "", fmt.Errorf("no workspace folder is open")
	}
	if id != workspaceID {
		return "", fmt.Errorf("workspace %s is not the open one", workspaceID)
	}
	return root, nil
}
