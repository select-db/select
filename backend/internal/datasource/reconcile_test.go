package datasource_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"backend/e2e"
	"backend/internal/cellar"
	"backend/internal/datasource/managed"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// reconcileFixture is a workspace over a test cellar, and the folder of the
// cellar's files.
type reconcileFixture struct {
	e2e.Fixture
	cellarDir string
}

func newReconcileFixture(t *testing.T) reconcileFixture {
	t.Helper()
	fixture := e2e.Setup(t)
	return reconcileFixture{Fixture: fixture, cellarDir: e2e.ServeCellar(t)}
}

func (f reconcileFixture) onCellar(id string) bool {
	_, err := os.Stat(filepath.Join(f.cellarDir, id+".db"))
	return err == nil
}

func (f reconcileFixture) rowState(t *testing.T, id string) (state string, exists bool) {
	t.Helper()
	err := f.Conn.QueryRow(`SELECT state FROM app.datasource WHERE id = $1`, id).Scan(&state)
	if err == sql.ErrNoRows {
		return "", false
	}
	require.NoError(t, err)
	return state, true
}

// orphan puts a database on the cellar that no row names, last written age ago.
func (f reconcileFixture) orphan(t *testing.T, age time.Duration) string {
	t.Helper()
	id := uuid.NewString()
	dsn := cellarclient.DSN(cellarclient.CellarID, id, f.Actor.WorkspaceID, 1<<20, 0)
	_, err := cellarclient.Create(context.Background(), dsn, "", "")
	require.NoError(t, err)
	old := time.Now().Add(-age)
	path := filepath.Join(f.cellarDir, id+".db")
	require.NoError(t, os.Chtimes(path, old, old))
	if _, err := os.Stat(path + "-wal"); err == nil {
		require.NoError(t, os.Chtimes(path+"-wal", old, old))
	}
	return id
}

func TestReconcilePurgesWhatIsDeleting(t *testing.T) {
	f := newReconcileFixture(t)
	deleted, kept := createNotes(t, f.Fixture), createNotes(t, f.Fixture)
	status, body := callAsOwner(t, f.Fixture, http.MethodDelete, "/datasources/"+deleted, nil)
	require.Equal(t, http.StatusNoContent, status, string(body))
	require.True(t, f.onCellar(deleted), "a delete only stops serving")

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 1, result.Purged)
	require.False(t, f.onCellar(deleted))
	_, exists := f.rowState(t, deleted)
	require.False(t, exists, "its row goes with its file")
	require.True(t, f.onCellar(kept))
	state, exists := f.rowState(t, kept)
	require.True(t, exists)
	require.Equal(t, "hot", state)
}

func TestReconcileLeavesAYoungOrphanAlone(t *testing.T) {
	f := newReconcileFixture(t)
	young := f.orphan(t, time.Hour)
	old := f.orphan(t, 25*time.Hour)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 1, result.Purged)
	require.True(t, f.onCellar(young), "a create writes the file before its row: wait a day")
	require.False(t, f.onCellar(old))
}

func TestReconcilePurgesTheDatabasesOfADeletedWorkspace(t *testing.T) {
	f := newReconcileFixture(t)
	id := createNotes(t, f.Fixture)
	_, err := f.Conn.Exec(`UPDATE app.workspace SET deleted_at = now() WHERE id = $1`, f.Actor.WorkspaceID)
	require.NoError(t, err)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 1, result.Purged)
	require.False(t, f.onCellar(id))
	_, exists := f.rowState(t, id)
	require.False(t, exists)
}

func TestReconcileLeavesAnotherCellarsDatabaseAlone(t *testing.T) {
	f := newReconcileFixture(t)
	id := createNotes(t, f.Fixture)
	_, err := f.Conn.Exec(`UPDATE app.datasource SET cellar_id = 'elsewhere', state = 'deleting' WHERE id = $1`, id)
	require.NoError(t, err)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Zero(t, result.Purged, "its own cellar decides")
	require.True(t, f.onCellar(id))
	_, exists := f.rowState(t, id)
	require.True(t, exists)
}

func TestReconcileDropsADeletingRowTheCellarDoesNotHold(t *testing.T) {
	f := newReconcileFixture(t)
	id := createNotes(t, f.Fixture)
	status, _ := callAsOwner(t, f.Fixture, http.MethodDelete, "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, status)
	_, err := managed.Reconcile(context.Background(), time.Now())
	require.NoError(t, err)

	missing := uuid.NewString()
	_, err = f.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
		VALUES ($1::uuid, $2::uuid, 'gone', 'sqlite', $3, 'deleting')`, missing, f.Actor.WorkspaceID, cellarclient.CellarID)
	require.NoError(t, err)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 1, result.RowsDropped)
	_, exists := f.rowState(t, missing)
	require.False(t, exists)
}

func TestReconcileStopsAtTheCapAndResumes(t *testing.T) {
	f := newReconcileFixture(t)
	const total = 55
	for range total {
		f.orphan(t, 48*time.Hour)
	}

	first, err := managed.Reconcile(context.Background(), time.Now())
	require.NoError(t, err)
	require.Equal(t, 50, first.Purged)
	require.True(t, first.CapHit)

	second, err := managed.Reconcile(context.Background(), time.Now())
	require.NoError(t, err)
	require.Equal(t, total-50, second.Purged)
	require.False(t, second.CapHit)
}

func TestReconcilePurgesNothingWhenPostgresFails(t *testing.T) {
	f := newReconcileFixture(t)
	deleted := createNotes(t, f.Fixture)
	status, _ := callAsOwner(t, f.Fixture, http.MethodDelete, "/datasources/"+deleted, nil)
	require.Equal(t, http.StatusNoContent, status)
	orphan := f.orphan(t, 48*time.Hour)

	_, err := f.Conn.Exec(`ALTER TABLE app.datasource RENAME TO datasource_away`)
	require.NoError(t, err)
	restore := func() { _, _ = f.Conn.Exec(`ALTER TABLE IF EXISTS app.datasource_away RENAME TO datasource`) }
	t.Cleanup(restore)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.Error(t, err)
	require.Zero(t, result.Purged)
	require.True(t, f.onCellar(deleted))
	require.True(t, f.onCellar(orphan), "no row could be read, so none is a reason")
}

func TestReconcilePurgesNothingWhenTheCellarListingFails(t *testing.T) {
	f := newReconcileFixture(t)
	id := createNotes(t, f.Fixture)
	status, _ := callAsOwner(t, f.Fixture, http.MethodDelete, "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, status)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	realURL := cellarclient.URL
	cellarclient.URL = failing.URL
	t.Cleanup(func() { cellarclient.URL = realURL })

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.Error(t, err)
	require.Zero(t, result.Purged)
	_, exists := f.rowState(t, id)
	require.True(t, exists, "a row the cellar could not confirm stays")
}

func TestReconcileRunsOnOneBackendAtATime(t *testing.T) {
	f := newReconcileFixture(t)
	f.orphan(t, 48*time.Hour)
	conn, err := f.Conn.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	var held bool
	require.NoError(t, conn.QueryRowContext(context.Background(), `SELECT pg_try_advisory_lock($1)`, int64(0x63656c6c5f726563)).Scan(&held))
	require.True(t, held)

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Zero(t, result.Purged, fmt.Sprintf("%+v", result))
}

// The row is written once, at creation. Without this, the workspace's byte quota
// counts a database that grew to its cap as the empty file it was.
func TestReconcileRecordsTheSizeOfALiveDatabase(t *testing.T) {
	f := newReconcileFixture(t)
	id := createNotes(t, f.Fixture)
	_, failure := e2e.Execute(t, f.Fixture, id, "CREATE TABLE blobs (b BLOB); INSERT INTO blobs SELECT zeroblob(5000000)")
	require.Nil(t, failure)
	var before int64
	require.NoError(t, f.Conn.QueryRow(`SELECT size_bytes FROM app.datasource WHERE id = $1`, id).Scan(&before))
	require.Less(t, before, int64(1<<20), "recorded when it was empty")

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 1, result.Resized)
	var after int64
	require.NoError(t, f.Conn.QueryRow(`SELECT size_bytes FROM app.datasource WHERE id = $1`, id).Scan(&after))
	require.GreaterOrEqual(t, after, int64(5_000_000))
}

// The cellar id is checked twice: by the database, when a managed row is
// written, and by cellar.ValidID, at start. They must agree, or a bad id gets
// past the start check and fails at the first create.
func TestValidIDAgreesWithTheDatabaseConstraint(t *testing.T) {
	fixture := e2e.Setup(t)
	for _, id := range []string{
		"staging", "prod", "a", "0", "cellar-1", "a-b-c", "9lives",
		"", "-a", "A", "Staging", "a.b", "127.0.0.1", "a_b", "a b", "a/b", "é", "a\n",
	} {
		_, err := fixture.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
			VALUES ($1::uuid, $2::uuid, 'x', 'sqlite', $3, 'hot')`, uuid.NewString(), fixture.Actor.WorkspaceID, id)
		require.Equalf(t, cellar.ValidID(id), err == nil, "cellar id %q: the database says %v", id, err)
	}
}

func TestReconcileStopsWhenPurgesKeepFailing(t *testing.T) {
	f := newReconcileFixture(t)
	for range 6 {
		f.orphan(t, 48*time.Hour)
	}
	// A cellar that lists its databases and refuses to remove any.
	inventory, err := json.Marshal(func() []map[string]any {
		entries, _ := os.ReadDir(f.cellarDir)
		var listed []map[string]any
		for _, entry := range entries {
			if id, ok := strings.CutSuffix(entry.Name(), ".db"); ok {
				listed = append(listed, map[string]any{"id": id, "size_bytes": 1, "modified_at": time.Now().Add(-48 * time.Hour)})
			}
		}
		return listed
	}())
	require.NoError(t, err)
	var purgeCalls atomic.Int32
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(inventory)
			return
		}
		purgeCalls.Add(1)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(refusing.Close)
	realURL := cellarclient.URL
	cellarclient.URL = refusing.URL
	t.Cleanup(func() { cellarclient.URL = realURL })

	result, err := managed.Reconcile(context.Background(), time.Now())

	require.Error(t, err)
	require.Equal(t, 3, result.Failed)
	require.EqualValues(t, 3, purgeCalls.Load(), "a cellar that refuses is not asked once per database")
}
