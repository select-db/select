package cellar

import (
	"context"
	"errors"
	"time"

	"github.com/benbjohnson/litestream"
)

// wakeWait is how long a statement waits for its database to be restored
// before it is answered errWaking; the restore goes on.
const wakeWait = 15 * time.Second

// wake returns once database id is on disk, restoring it if it is cold.
func (databases *Databases) wake(ctx context.Context, id, path string) error {
	databases.mu.Lock()
	_, onDisk := databases.onDisk[id]
	databases.mu.Unlock()
	if onDisk {
		return nil
	}
	restore := databases.restores.DoChan(id, func() (any, error) {
		return nil, databases.restoreLatest(id, path)
	})
	select {
	case result := <-restore:
		return result.Err
	case <-time.After(wakeWait):
		return errWaking
	case <-ctx.Done():
		return ctx.Err()
	}
}

// restoreLatest runs outside any request, so a caller that stops waiting does
// not cancel the restore the next caller needs.
func (databases *Databases) restoreLatest(id, path string) error {
	// A caller that saw the database cold as the last restore ended gets here after it.
	databases.mu.Lock()
	_, onDisk := databases.onDisk[id]
	databases.mu.Unlock()
	if onDisk {
		return nil
	}
	err := databases.restoreAt(context.Background(), id, time.Time{}, path)
	if errors.Is(err, errNoCopyAtTime) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	databases.mu.Lock()
	databases.onDisk[id] = &database{id: id, path: path, lastUsed: time.Now()}
	databases.mu.Unlock()
	return nil
}

// restoreAt writes database id as it was at the given time, or its latest
// state when at is zero, to a new file at path.
func (databases *Databases) restoreAt(ctx context.Context, id string, at time.Time, path string) error {
	client, err := databases.bucketClient(id)
	if err != nil {
		return err
	}
	options := litestream.NewRestoreOptions()
	options.OutputPath = path
	options.Timestamp = at
	err = litestream.NewReplicaWithClient(nil, client).Restore(ctx, options)
	// The bucket answers the same for no copy that old and for no database.
	if errors.Is(err, litestream.ErrTxNotAvailable) {
		return errNoCopyAtTime
	}
	return err
}
