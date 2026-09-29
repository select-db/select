package cellar

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// evictAll rests and evicts every database, as a long idle cellar short of disk does.
func (cellar testCellar) evictAll(t *testing.T) {
	t.Helper()
	cellar.databases.rest(context.Background(), time.Now().Add(restAfter))
	cellar.databases.evict(shortFor(100))
}

func TestWakeRestoresAnEvictedDatabase(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	// Not synced by hand: resting sends the last writes.
	cellar.exec(t, id, "INSERT INTO note VALUES ('c')")
	cellar.evictAll(t)
	require.NoFileExists(t, cellar.dir+"/"+id+".db")

	_, err := cellar.databases.use(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, 3, cellar.countNotes(t, id))
	require.NotNil(t, cellar.databases.onDisk[id].replicating)

	cellar.exec(t, id, "INSERT INTO note VALUES ('d')")
	cellar.evictAll(t)
	_, err = cellar.databases.use(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, 4, cellar.countNotes(t, id), "a woken database replicates again")
}

func TestWakeIsSharedByConcurrentCallers(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	cellar.evictAll(t)

	// A second restore would fail: the file it writes already exists.
	var callers sync.WaitGroup
	errs := make([]error, 8)
	for caller := range errs {
		callers.Go(func() {
			_, errs[caller] = cellar.databases.use(context.Background(), id)
		})
	}
	callers.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 2, cellar.countNotes(t, id))
}
