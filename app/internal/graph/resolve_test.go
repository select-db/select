package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// resolvedFolder reads a folder's state under the lock the prefetch writes it
// under, so a poll for it is not a race with the goroutine doing the reading.
func resolvedFolder(g *Graph, uri string) (*FolderNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	folder, ok := g.lookup(uri).(*FolderNode)
	if !ok {
		return nil, false
	}
	return folder, folder.Resolved
}

// awaitResolved waits for the prefetch, which runs off the caller's path.
func awaitResolved(t *testing.T, g *Graph, uri string) *FolderNode {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if folder, ok := resolvedFolder(g, uri); ok {
			return folder
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s was never read", uri)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Opening a folder reads the folders inside it too, so clicking one of them
// shows its files at once rather than after another round trip. One level, not
// the whole subtree: the level below that stays unread until it is asked for.
func TestResolveFolder_ReadsOneLevelAhead(t *testing.T) {
	withTempAppDataDir(t)

	const workspaceID = "ws-1"
	rootURI := fmt.Sprintf("selectdb://workspaces/%s", workspaceID)
	workspaceRoot := openTestWorkspace(t, workspaceID)

	// workspaces/ws-1/reports/{monthly.sql, drafts/{draft.sql, archive/old.sql}}
	archive := filepath.Join(workspaceRoot, "reports", "drafts", "archive")
	if err := os.MkdirAll(archive, 0o700); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}

	files := map[string]string{
		filepath.Join(workspaceRoot, "reports", "monthly.sql"):         "SELECT 1;",
		filepath.Join(workspaceRoot, "reports", "drafts", "draft.sql"): "SELECT 2;",
		filepath.Join(archive, "old.sql"):                              "SELECT 3;",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	g := &Graph{
		WorkspaceGraph: &WorkspaceNode{
			ID:          workspaceID,
			Type:        "workspace",
			Name:        "MyWorkspace",
			Folders:     []*FolderNode{},
			DBInstances: []*DBInstanceNode{},
		},
	}

	fsCtx := NewWorkspaceFSFromRoot(workspaceID, workspaceRoot)
	if err := g.buildWorkspaceGraphFromFS(fsCtx); err != nil {
		t.Fatalf("buildWorkspaceGraphFromFS: %v", err)
	}

	if _, err := g.ResolveFolder(rootURI + "/reports"); err != nil {
		t.Fatalf("ResolveFolder: %v", err)
	}

	drafts := awaitResolved(t, g, rootURI+"/reports/drafts")

	g.mu.RLock()
	draftFiles := len(drafts.Files)
	firstName := ""
	if draftFiles > 0 {
		firstName = drafts.Files[0].Name
	}
	g.mu.RUnlock()

	_, archiveResolved := resolvedFolder(g, rootURI+"/reports/drafts/archive")

	if draftFiles != 1 || firstName != "draft.sql" {
		t.Errorf("drafts contents mismatch: %d files, first %q", draftFiles, firstName)
	}
	if archiveResolved {
		t.Errorf("archive is two levels down and should stay unread")
	}
}
