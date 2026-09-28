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
func (d *Databases) wake(ctx context.Context, id, path string) error {
	d.mu.Lock()
	_, onDisk := d.onDisk[id]
	d.mu.Unlock()
	if onDisk {
		return nil
	}
	restore := d.restores.DoChan(id, func() (any, error) {
		return nil, d.restoreLatest(id, path)
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
func (d *Databases) restoreLatest(id, path string) error {
	// A caller that saw the database cold as the last restore ended gets here after it.
	d.mu.Lock()
	_, onDisk := d.onDisk[id]
	d.mu.Unlock()
	if onDisk {
		return nil
	}
	err := d.restoreAt(context.Background(), id, time.Time{}, path)
	if err == errNoCopyAtTime {
		return errNotFound
	}
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.onDisk[id] = &database{id: id, path: path, lastUsed: time.Now()}
	d.mu.Unlock()
	return nil
}

// restoreAt writes database id as it was at the given time, or its latest
// state when at is zero, to a new file at path.
func (d *Databases) restoreAt(ctx context.Context, id string, at time.Time, path string) error {
	client, err := d.replicaClient(id)
	if err != nil {
		return err
	}
	options := litestream.NewRestoreOptions()
	options.OutputPath = path
	options.Timestamp = at
	err = litestream.NewReplicaWithClient(nil, client).Restore(ctx, options)
	// The replica answers the same for no copy that old and for no database.
	if errors.Is(err, litestream.ErrTxNotAvailable) {
		return errNoCopyAtTime
	}
	return err
}
