package system

import (
	"path/filepath"
	"testing"

	"os"

	"selectDb/backend/db/generated"
	"selectDb/backend/graph"

	"github.com/fsnotify/fsnotify"
)

// newTestWorkspaceFS creates a temporary workspace root and returns a
// WorkspaceFS plus the absolute workspace root path.
func newTestWorkspaceFS(t *testing.T) (*graph.WorkspaceFS, string) {
	t.Helper()

	appRoot := t.TempDir()
	const workspaceID = "ws-1"

	workspaceRoot := filepath.Join(appRoot, "workspaces", workspaceID)
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}

	return graph.NewWorkspaceFSFromRoot(workspaceID, workspaceRoot), workspaceRoot
}

func TestHandleDBConfigEvent_Insert(t *testing.T) {
	fsCtx, workspaceRoot := newTestWorkspaceFS(t)

	// Create a db instance folder with db.config.json.
	dbDir := filepath.Join(workspaceRoot, "folder-db-1", "db1")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	dbConfigPath := filepath.Join(dbDir, "db.config.json")
	dbConfig := `{
  "version": 1,
  "id": "db-1",
  "name": "DB1",
  "db_type": "sqlite",
  "dsn": "file:test.db",
  "workspace_id": "ws-1"
}`
	if err := os.WriteFile(dbConfigPath, []byte(dbConfig), 0o600); err != nil {
		t.Fatalf("write db.config.json: %v", err)
	}

	var commits []generated.MutationCommit
	s := &System{
		emitHook: func(c generated.MutationCommit) {
			commits = append(commits, c)
		},
	}

	ev := fsnotify.Event{
		Name: dbConfigPath,
		Op:   fsnotify.Create,
	}

	s.handleDBConfigEvent(ev, "user-1", fsCtx)

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}

	c := commits[0]
	if c.TableName != "db_instance" || c.Operation != "insert" {
		t.Fatalf("unexpected commit: %+v", c)
	}

	if c.ObjectID != "db-1" {
		t.Errorf("ObjectID mismatch: got %q want 'db-1'", c.ObjectID)
	}

	payload, ok := c.Payload.(graph.DBInstanceDTO)
	if !ok {
		t.Fatalf("payload type mismatch: %T", c.Payload)
	}

	if *payload.ID != "db-1" {
		t.Errorf("payload.id mismatch: got %v want 'db-1'", payload.ID)
	}
	if payload.Name == nil || *payload.Name != "DB1" {
		t.Errorf("payload.name mismatch: got %v want 'DB1'", payload.Name)
	}
	if payload.FolderID == nil || *payload.FolderID != fsCtx.URI("folder-db-1") {
		t.Errorf("payload.folder_id mismatch: got %v want %v", payload.FolderID, fsCtx.URI("folder-db-1"))
	}
}

func TestHandleDBConfigEvent_Delete(t *testing.T) {
	fsCtx, workspaceRoot := newTestWorkspaceFS(t)

	// Path where db.config.json used to live.
	dbDir := filepath.Join(workspaceRoot, "folder-db-1", "db1")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	dbConfigPath := filepath.Join(dbDir, "db.config.json")

	var commits []generated.MutationCommit
	s := &System{
		emitHook: func(c generated.MutationCommit) {
			commits = append(commits, c)
		},
	}

	ev := fsnotify.Event{
		Name: dbConfigPath,
		Op:   fsnotify.Remove,
	}

	s.handleDBConfigEvent(ev, "user-1", fsCtx)

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}

	c := commits[0]
	if c.TableName != "db_instance" || c.Operation != "delete" {
		t.Fatalf("unexpected commit: %+v", c)
	}

	expectedURI := fsCtx.URI("folder-db-1/db1")
	if c.ObjectID != expectedURI {
		t.Errorf("ObjectID mismatch: got %q want %q", c.ObjectID, expectedURI)
	}
}

func TestHandleMetadataEvent_Update(t *testing.T) {
	fsCtx, workspaceRoot := newTestWorkspaceFS(t)

	// Create a file and its metadata sidecar.
	filePath := filepath.Join(workspaceRoot, "file.sql")
	if err := os.WriteFile(filePath, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	metaPath := filePath + ".metadata.json"
	meta := `{"databases":[{"name":"db-1","id":"db-1"}]}`
	if err := os.WriteFile(metaPath, []byte(meta), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	var commits []generated.MutationCommit
	s := &System{
		emitHook: func(c generated.MutationCommit) {
			commits = append(commits, c)
		},
	}

	ev := fsnotify.Event{
		Name: metaPath,
		Op:   fsnotify.Write,
	}

	s.handleMetadataEvent(ev, "user-1", fsCtx)

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}

	c := commits[0]
	if c.TableName != "file" || c.Operation != "update" {
		t.Fatalf("unexpected commit: %+v", c)
	}

	expectedURI := fsCtx.URI("file.sql")
	if c.ObjectID != expectedURI {
		t.Errorf("ObjectID mismatch: got %q want %q", c.ObjectID, expectedURI)
	}

	payload, ok := c.Payload.(graph.FileDTO)
	if !ok {
		t.Fatalf("payload type mismatch: %T", c.Payload)
	}
	if payload.Databases == nil || len(*payload.Databases) != 1 || (*payload.Databases)[0].ID != "db-1" {
		t.Errorf("payload.databases mismatch: got %v want [{name:\"db-1\",id:\"db-1\"}]", payload.Databases)
	}
}

func TestHandleFSEvent_FileInsert(t *testing.T) {
	fsCtx, workspaceRoot := newTestWorkspaceFS(t)

	// Create a file on disk.
	filePath := filepath.Join(workspaceRoot, "file-root.sql")
	if err := os.WriteFile(filePath, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var commits []generated.MutationCommit
	s := &System{
		emitHook: func(c generated.MutationCommit) {
			commits = append(commits, c)
		},
	}

	ev := fsnotify.Event{
		Name: filePath,
		Op:   fsnotify.Create,
	}

	s.handleFSEvent(ev, "user-1", fsCtx)

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}

	c := commits[0]
	if c.TableName != "file" || c.Operation != "insert" {
		t.Fatalf("unexpected commit: %+v", c)
	}

	expectedURI := fsCtx.URI("file-root.sql")
	if c.ObjectID != expectedURI {
		t.Errorf("ObjectID mismatch: got %q want %q", c.ObjectID, expectedURI)
	}

	payload, ok := c.Payload.(graph.FileDTO)
	if !ok {
		t.Fatalf("payload type mismatch: %T", c.Payload)
	}
	if payload.ID == nil || *payload.ID != expectedURI {
		t.Errorf("payload.id mismatch: got %v want %q", payload.ID, expectedURI)
	}
	if payload.Name == nil || *payload.Name != "file-root.sql" {
		t.Errorf("payload.name mismatch: got %v want %v", payload.Name, "file-root.sql")
	}
	if payload.FolderID == nil || *payload.FolderID != fsCtx.URI("") {
		t.Errorf("payload.folder_id mismatch: got %v want %v", payload.FolderID, fsCtx.URI(""))
	}
}
