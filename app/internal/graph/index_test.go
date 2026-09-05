package graph

import (
	"testing"
)

func indexedGraph() *Graph {
	g := setupGraph()
	g.ensureIndex()
	return g
}

func TestIndex_LookupCoversTheTree(t *testing.T) {
	g := indexedGraph()

	root, ok := g.lookup("root").(*FolderNode)
	if !ok {
		t.Fatalf("root folder not indexed, got %T", g.lookup("root"))
	}

	nested := &FolderNode{ID: "nested", FolderID: "root"}
	nested.AddChild(&FileNode{ID: "deep-file", FolderID: "nested"})
	root.AddChild(nested)
	g.index.addSubtree(nested)

	if _, ok := g.lookup("deep-file").(*FileNode); !ok {
		t.Errorf("file below a subtree not indexed")
	}
	if g.lookup("missing") != nil {
		t.Errorf("unknown ID resolved to a node")
	}
}

func TestIndex_DbInstanceAnswersToIDAndURI(t *testing.T) {
	g := indexedGraph()

	db := &DBInstanceNode{
		ID:          "db-1",
		URI:         "selectdb://workspaces/ws-1/db1",
		FolderID:    "root",
		WorkspaceID: "ws-1",
	}
	g.attach(db)

	if g.lookup("db-1") != Node(db) {
		t.Errorf("db instance not found by ID")
	}
	if g.lookup("selectdb://workspaces/ws-1/db1") != Node(db) {
		t.Errorf("db instance not found by URI")
	}

	// A db instance hangs from both its folder and the workspace's flat list,
	// and detaching has to clear both.
	root, _ := g.lookup("root").(*FolderNode)
	if len(root.DBInstances) != 1 || len(g.WorkspaceGraph.DBInstances) != 1 {
		t.Fatalf("expected the instance under both parents, got folder=%d workspace=%d",
			len(root.DBInstances), len(g.WorkspaceGraph.DBInstances))
	}

	if !g.detachByIDs([]string{"db-1"}) {
		t.Fatalf("expected detach to report a removal")
	}
	if len(root.DBInstances) != 0 || len(g.WorkspaceGraph.DBInstances) != 0 {
		t.Errorf("instance left behind: folder=%d workspace=%d",
			len(root.DBInstances), len(g.WorkspaceGraph.DBInstances))
	}
	if g.lookup("db-1") != nil || g.lookup("selectdb://workspaces/ws-1/db1") != nil {
		t.Errorf("detached instance still in the index")
	}
}

func TestIndex_DetachDropsTheWholeSubtree(t *testing.T) {
	g := indexedGraph()

	folder := &FolderNode{ID: "folder-1", FolderID: "root"}
	folder.AddChild(&FileNode{ID: "file-1", FolderID: "folder-1"})
	g.attach(folder)

	if g.lookup("file-1") == nil {
		t.Fatalf("child file not indexed on attach")
	}

	g.detachByIDs([]string{"folder-1"})

	if g.lookup("folder-1") != nil {
		t.Errorf("detached folder still in the index")
	}
	if g.lookup("file-1") != nil {
		t.Errorf("child of a detached folder still in the index")
	}
}

func TestIndex_SchemaItemsAreNotIndexed(t *testing.T) {
	g := indexedGraph()

	db := &DBInstanceNode{ID: "db-1", URI: "db-1-uri", FolderID: "root", WorkspaceID: "ws-1"}
	db.AddChild(&DBInstanceItemNode{ID: "public", ParentID: "db-1"})
	g.attach(db)

	// Schema items are replaced wholesale by a schema load without going
	// through the graph package, so they are looked up by walking the instance.
	if g.lookup("public") != nil {
		t.Errorf("schema item should not be in the index")
	}
	if g.FindDbItemNodeById("db-1", "public") == nil {
		t.Errorf("schema item should still be reachable through FindDbItemNodeById")
	}
}
