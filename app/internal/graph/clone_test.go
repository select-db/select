package graph

import (
	"reflect"
	"testing"
)

// fullTree is a workspace with every field of every node set, so a field added
// later and left out of Clone shows up as a difference rather than as nothing.
func fullTree() *WorkspaceNode {
	file := &FileNode{
		ID: "file", URI: "selectdb://f", Type: "file", Name: "q.sql", FolderID: "folder",
		Databases:      []DatabaseRef{{ID: "db", Name: "warehouse"}},
		QueryResults:   map[string]*QueryResult{"db": {Id: "r", Columns: []string{"a"}}},
		PlanResults:    map[string]*ExplainResult{"db": {}},
		ExplainResults: map[string]*ExplainResult{"db": {}},
		Badges:         []string{"badge"},
	}
	item := &DBInstanceItemNode{
		ID: "item", URI: "selectdb://i", Type: "schema", Name: "public", Path: "public",
		Badges: []string{"badge"}, Metadata: map[string]string{"k": "v"},
		ParentID: "db", Children: []*DBInstanceItemNode{{ID: "child", Name: "orders"}},
	}
	db := &DBInstanceNode{
		ID: "db", URI: "selectdb://db", Type: "db_instance", Name: "warehouse",
		DBType: "postgresql", DSN: "postgres://", Proxified: true,
		SSH:         &DBInstanceSSHConfig{Enabled: true, Host: "h", Port: 22, User: "u", AuthMethod: "agent", Password: "p", PrivateKey: "k"},
		FolderID:    "folder",
		WorkspaceID: "ws",
		Children:    []*DBInstanceItemNode{item},
		Files:       []*FileNode{file},
		Folders:     []*FolderNode{{ID: "inner", Name: "inner"}},
	}
	folder := &FolderNode{
		ID: "folder", URI: "selectdb://folder", Type: "folder", Name: "queries", FolderID: "root",
		Files: []*FileNode{file}, Folders: []*FolderNode{{ID: "nested", Name: "nested"}},
		DBInstances: []*DBInstanceNode{db},
		Resolved:    true,
		Variables:   map[string]string{"HOST": "localhost"},
		Badges:      []string{"badge"},
	}
	return &WorkspaceNode{
		ID: "ws", Type: "workspace", Name: "analytics", IsOwner: true, Logo: "png",
		StatementTimeoutMs: 1000, MaxResultSizeMB: 10,
		User:        &UserNode{ID: "u", Type: "user", Name: "Sam"},
		Folders:     []*FolderNode{folder},
		DBInstances: []*DBInstanceNode{db},
	}
}

func TestCloneCarriesEveryField(t *testing.T) {
	original := fullTree()
	requireNoZeroField(t, original, "WorkspaceNode")
	requireNoZeroField(t, original.Folders[0], "FolderNode")
	requireNoZeroField(t, original.Folders[0].Files[0], "FileNode")
	requireNoZeroField(t, original.DBInstances[0], "DBInstanceNode")
	requireNoZeroField(t, original.DBInstances[0].Children[0], "DBInstanceItemNode")
	requireNoZeroField(t, original.User, "UserNode")

	if clone := original.Clone(); !reflect.DeepEqual(original, clone) {
		t.Fatal("clone differs from the tree it was taken from")
	}
}

// TestCloneOutlivesTheTree is the reason Clone exists: the frontend holds the
// copy while the watcher keeps writing to the tree.
func TestCloneOutlivesTheTree(t *testing.T) {
	original := fullTree()
	clone := original.Clone()

	folder := original.Folders[0]
	folder.Files = append(folder.Files, &FileNode{ID: "late"})
	folder.DBInstances = nil
	folder.Variables["HOST"] = "elsewhere"
	folder.Badges[0] = "rewritten"

	cloned := clone.Folders[0]
	if len(cloned.Files) != 1 {
		t.Errorf("files: got %d, want 1", len(cloned.Files))
	}
	if len(cloned.DBInstances) != 1 {
		t.Errorf("databases: got %d, want 1", len(cloned.DBInstances))
	}
	if cloned.Variables["HOST"] != "localhost" {
		t.Errorf("variables: got %q", cloned.Variables["HOST"])
	}
	if cloned.Badges[0] != "badge" {
		t.Errorf("badges: got %q", cloned.Badges[0])
	}
}

// requireNoZeroField keeps the fixtures above honest: a field nobody sets here
// would compare equal whether or not Clone carries it.
func requireNoZeroField(t *testing.T, node any, name string) {
	t.Helper()

	v := reflect.ValueOf(node).Elem()
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Errorf("%s.%s is not set in the test fixture, so Clone is not checked for it",
				name, v.Type().Field(i).Name)
		}
	}
}
