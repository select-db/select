package cellar

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// shortFor is a disk that stays short for the next checks times it is asked.
func shortFor(checks int) func() bool {
	return func() bool {
		checks--
		return checks >= 0
	}
}

func TestRestAfterIdle(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)

	databases.rest(context.Background(), time.Now())
	require.NotNil(t, databases.onDisk[id].replicating, "a database just used keeps replicating")

	databases.rest(context.Background(), time.Now().Add(restAfter))
	require.Nil(t, databases.onDisk[id].replicating)
	require.Equal(t, 2, cellar.countNotes(t, id), "a resting database stays on disk")
}

func TestEvictLeastRecentlyUsedRestingFirst(t *testing.T) {
	cellar := newTestCellar(t)
	oldestID, olderID, busyID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{oldestID, olderID, busyID} {
		cellar.createNotes(t, id)
	}
	databases.onDisk[oldestID].lastUsed = time.Now().Add(-2 * time.Hour)
	databases.onDisk[olderID].lastUsed = time.Now().Add(-time.Hour)
	databases.rest(context.Background(), time.Now())

	databases.evict(shortFor(1))
	require.NoFileExists(t, cellar.dir+"/"+oldestID+".db")
	require.FileExists(t, cellar.dir+"/"+olderID+".db")

	databases.evict(shortFor(5))
	require.NoFileExists(t, cellar.dir+"/"+olderID+".db")
	require.FileExists(t, cellar.dir+"/"+busyID+".db", "a replicating database is never evicted")
}

func TestRestSendsEveryWriteToTheBucket(t *testing.T) {
	cellar := newTestCellar(t)
	id, copyID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, id)
	// Not synced by hand: rest must send it.
	cellar.exec(t, id, "INSERT INTO note VALUES ('c')")

	databases.rest(context.Background(), time.Now().Add(restAfter))
	require.Nil(t, databases.onDisk[id].replicating)
	require.NoError(t, databases.restoreAt(context.Background(), id, time.Time{}, filepath.Join(cellar.dir, copyID+".db")))
	require.Equal(t, 3, cellar.countNotes(t, copyID), "a resting database is whole in the bucket")
}

func TestRestKeepsADatabaseUsedWhileItSyncs(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	database := databases.onDisk[id]
	idleSince := time.Now().Add(-restAfter)

	// A use that lands between the sync and the lock moves lastUsed past the rest's clock.
	database.lastUsed = time.Now()
	databases.restOne(context.Background(), database, database.replicating, idleSince.Add(restAfter))
	require.NotNil(t, database.replicating)
}

func TestRestSkipsADatabaseRemovedWhileItSyncs(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	database := databases.onDisk[id]
	replicating := database.replicating
	require.NoError(t, databases.remove(context.Background(), id))

	databases.restOne(context.Background(), database, replicating, time.Now().Add(restAfter))
	require.NotContains(t, databases.onDisk, id, "a removed database is not brought back")
}

func TestRestKeepsReplicatingWhenTheBucketFails(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	cellar.sync(t, id)
	// A file where the bucket's folders go: every upload fails, even as root.
	require.NoError(t, os.RemoveAll(filepath.Join(cellar.bucketDir, "dbs")))
	require.NoError(t, os.WriteFile(filepath.Join(cellar.bucketDir, "dbs"), nil, 0o600))
	// Mended before cleanup, whose last sync would otherwise retry for 30s.
	t.Cleanup(func() { _ = os.Remove(filepath.Join(cellar.bucketDir, "dbs")) })
	cellar.exec(t, id, "INSERT INTO note VALUES ('c')")

	databases.rest(context.Background(), time.Now().Add(restAfter))
	require.NotNil(t, databases.onDisk[id].replicating, "not whole in the bucket, so not resting")
	databases.evict(shortFor(10))
	require.FileExists(t, filepath.Join(cellar.dir, id+".db"), "and never evicted")
}

func TestEvictRemovesEveryFileOfTheDatabase(t *testing.T) {
	cellar := newTestCellar(t)
	evictedID, keptID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, evictedID)
	cellar.createNotes(t, keptID)
	databases.onDisk[evictedID].lastUsed = time.Now().Add(-time.Hour)
	databases.rest(context.Background(), time.Now().Add(restAfter))

	databases.evict(shortFor(1))
	entries, err := os.ReadDir(cellar.dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), evictedID, "%s left behind", entry.Name())
	}
	require.NotContains(t, databases.onDisk, evictedID)
	require.Equal(t, 2, cellar.countNotes(t, keptID))
}

func TestEvictStopsWhenNothingRests(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)

	databases.evict(func() bool { return true })
	require.FileExists(t, filepath.Join(cellar.dir, id+".db"), "a disk that stays short never evicts a replicating database")
}

func TestFreeShare(t *testing.T) {
	share := freeShare(t.TempDir())
	require.Greater(t, share, 0.0)
	require.LessOrEqual(t, share, 1.0)
	require.Equal(t, 1.0, freeShare(filepath.Join(t.TempDir(), "missing")), "a failed check never evicts")
}
