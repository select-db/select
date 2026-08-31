package system

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"os"

	"selectDb/internal/db/generated"
	"selectDb/internal/graph"

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

// watchedPaths returns the watcher's registrations that live under root.
func watchedPaths(t *testing.T, watcher *fsnotify.Watcher, root string) []string {
	t.Helper()

	var paths []string
	for _, p := range watcher.WatchList() {
		if p == root || strings.HasPrefix(p, root+string(os.PathSeparator)) {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	return paths
}

// waitForEvent drains events until one names path, or the timeout expires.
func waitForEvent(t *testing.T, watcher *fsnotify.Watcher, path string, timeout time.Duration) bool {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return false
			}
			if ev.Name == path {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// A renamed directory keeps its watch, but the watcher goes on reporting it
// under the old path, so every registration below it goes stale.
func TestResyncWatches_ReplacesStalePathsAfterRename(t *testing.T) {
	_, workspaceRoot := newTestWorkspaceFS(t)

	oldDir := filepath.Join(workspaceRoot, "db-e731d451")
	if err := os.MkdirAll(filepath.Join(oldDir, "sub", "deep"), 0o700); err != nil {
		t.Fatalf("mkdir db dirs: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	addWatches(watcher, workspaceRoot)

	newDir := filepath.Join(workspaceRoot, "analytics")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatalf("rename db dir: %v", err)
	}

	resyncWatches(watcher, workspaceRoot)

	want := []string{
		workspaceRoot,
		newDir,
		filepath.Join(newDir, "sub"),
		filepath.Join(newDir, "sub", "deep"),
	}
	slices.Sort(want)

	got := watchedPaths(t, watcher, workspaceRoot)
	if !slices.Equal(got, want) {
		t.Fatalf("watch list mismatch after rename:\n got %v\nwant %v", got, want)
	}
}

// Re-adding on its own is not enough: the old and new paths are the same
// directory, so the stale registration has to be dropped first or events keep
// arriving under the pre-rename path.
func TestResyncWatches_EventsCarryNewPathAfterRename(t *testing.T) {
	_, workspaceRoot := newTestWorkspaceFS(t)

	oldDir := filepath.Join(workspaceRoot, "db-e731d451")
	if err := os.MkdirAll(filepath.Join(oldDir, "sub"), 0o700); err != nil {
		t.Fatalf("mkdir db dirs: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	addWatches(watcher, workspaceRoot)

	newDir := filepath.Join(workspaceRoot, "analytics")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatalf("rename db dir: %v", err)
	}

	resyncWatches(watcher, workspaceRoot)

	created := filepath.Join(newDir, "sub", "query.sql")
	if err := os.WriteFile(created, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file in renamed dir: %v", err)
	}

	if !waitForEvent(t, watcher, created, 3*time.Second) {
		t.Fatalf("no event naming %q: the watch is still reporting the pre-rename path", created)
	}
}
