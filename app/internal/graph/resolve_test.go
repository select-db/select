package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

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

	drafts, _ := g.lookup(rootURI + "/reports/drafts").(*FolderNode)
	if drafts == nil {
		t.Fatalf("drafts folder is not in the graph")
	}
	if !drafts.Resolved {
		t.Errorf("drafts should be read with the folder holding it")
	}
	if len(drafts.Files) != 1 || drafts.Files[0].Name != "draft.sql" {
		t.Errorf("drafts contents mismatch: %+v", drafts.Files)
	}

	archiveNode, _ := g.lookup(rootURI + "/reports/drafts/archive").(*FolderNode)
	if archiveNode == nil {
		t.Fatalf("archive folder is not in the graph")
	}
	if archiveNode.Resolved {
		t.Errorf("archive is two levels down and should stay unread")
	}
}
