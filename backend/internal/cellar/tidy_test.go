package cellar

import (
	"context"
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

	cellar.databases.rest(context.Background(), time.Now())
	require.NotNil(t, cellar.databases.onDisk[id].replicating, "a database just used keeps replicating")

	cellar.databases.rest(context.Background(), time.Now().Add(restAfter))
	require.Nil(t, cellar.databases.onDisk[id].replicating)
	require.Equal(t, 2, cellar.countNotes(t, id), "a resting database stays on disk")
}

func TestEvictLeastRecentlyUsedRestingFirst(t *testing.T) {
	cellar := newTestCellar(t)
	oldestID, olderID, busyID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{oldestID, olderID, busyID} {
		cellar.createNotes(t, id)
	}
	cellar.databases.onDisk[oldestID].lastUsed = time.Now().Add(-2 * time.Hour)
	cellar.databases.onDisk[olderID].lastUsed = time.Now().Add(-time.Hour)
	cellar.databases.rest(context.Background(), time.Now())

	cellar.databases.evict(shortFor(1))
	require.NoFileExists(t, cellar.dir+"/"+oldestID+".db")
	require.FileExists(t, cellar.dir+"/"+olderID+".db")

	cellar.databases.evict(shortFor(5))
	require.NoFileExists(t, cellar.dir+"/"+olderID+".db")
	require.FileExists(t, cellar.dir+"/"+busyID+".db", "a replicating database is never evicted")
}
