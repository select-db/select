package cellar

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOpenDatabasesReplicatesWhatIsOnDisk(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	require.NoError(t, os.WriteFile(filepath.Join(cellar.dir, ".tmp-"+uuid.NewString()+".db"), nil, 0o600))
	require.NoError(t, cellar.databases.Close(context.Background()))

	reopened, err := OpenDatabases(cellar.dir, cellar.bucketDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close(context.Background()) })
	require.Len(t, reopened.onDisk, 1, "WAL files and temporary copies are not databases")
	require.NotNil(t, reopened.onDisk[id].replicating, "a write the last run had not sent reaches the bucket")
}

func TestCloseSendsTheLastWrites(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	cellar.exec(t, id, "INSERT INTO note VALUES ('c')")

	require.NoError(t, cellar.databases.Close(context.Background()))
	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	require.NoError(t, cellar.databases.restoreAt(context.Background(), id, time.Time{}, restoredPath))
	restored, err := sql.Open("sqlite", restoredPath)
	require.NoError(t, err)
	defer restored.Close()
	var count int
	require.NoError(t, restored.QueryRow("SELECT count(*) FROM note").Scan(&count))
	require.Equal(t, 3, count, "the write made just before Close is in the bucket")
}

func TestUse(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	cellar.databases.rest(context.Background(), time.Now().Add(restAfter))
	require.Nil(t, cellar.databases.onDisk[id].replicating)
	before := cellar.databases.onDisk[id].lastUsed

	path, err := cellar.databases.use(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cellar.dir, id+".db"), path)
	require.NotNil(t, cellar.databases.onDisk[id].replicating, "a resting database replicates again once used")
	require.True(t, cellar.databases.onDisk[id].lastUsed.After(before))
}

func TestUseMissingDatabase(t *testing.T) {
	cellar := newTestCellar(t)

	_, err := cellar.databases.use(context.Background(), uuid.NewString())
	require.ErrorIs(t, err, errNotFound)
	_, err = cellar.databases.use(context.Background(), "../escape")
	require.Error(t, err, "an id that is not a uuid")
}

func TestAddRefusesATakenID(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	tempPath := filepath.Join(cellar.dir, ".tmp-"+uuid.NewString()+".db")
	require.NoError(t, execTrusted(context.Background(), tempPath, "rwc", "PRAGMA journal_mode = WAL"))

	_, err := cellar.databases.add(id, tempPath)
	require.ErrorIs(t, err, errAlreadyExists)
	require.Equal(t, 2, cellar.countNotes(t, id), "the database in place is kept")
}

func TestRemove(t *testing.T) {
	cellar := newTestCellar(t)
	onDiskID, coldID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, onDiskID)
	cellar.createNotes(t, coldID)
	cellar.sync(t, onDiskID)
	cellar.evictAll(t)
	_, err := cellar.databases.use(context.Background(), onDiskID)
	require.NoError(t, err)

	for _, id := range []string{onDiskID, coldID} {
		require.NoError(t, cellar.databases.remove(context.Background(), id))
		require.NotContains(t, cellar.databases.onDisk, id)
		require.NoFileExists(t, filepath.Join(cellar.dir, id+".db"))
		require.NoDirExists(t, filepath.Join(cellar.dir, "."+id+".db-litestream"))
		require.NoDirExists(t, filepath.Join(cellar.bucketDir, "dbs", id), "a cold database leaves the bucket too")
	}
	require.NoError(t, cellar.databases.remove(context.Background(), uuid.NewString()), "removing an unknown id is not an error")
}

func TestBucketClientRefusesAnIDThatIsNotAUUID(t *testing.T) {
	cellar := newTestCellar(t)

	_, err := cellar.databases.bucketClient("../other")
	require.Error(t, err, "an id names a folder of the bucket")
}
