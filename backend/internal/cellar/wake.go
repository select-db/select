package cellar

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/benbjohnson/litestream"
)

// wakeWait is how long a statement waits for its database to be restored
// before it is answered errWaking; the restore goes on.
const wakeWait = 15 * time.Second

// wake is one restore of a cold database, shared by every caller waiting on it.
type wake struct {
	done chan struct{}
	err  error
}

// wake returns once database id is on disk, restoring it if it is cold.
func (d *Databases) wake(ctx context.Context, id string) error {
	d.mu.Lock()
	_, onDisk := d.lastUsed[id]
	restore := d.waking[id]
	if !onDisk && restore == nil {
		restore = &wake{done: make(chan struct{})}
		d.waking[id] = restore
		go d.restoreLatest(id, restore)
	}
	d.mu.Unlock()
	if restore == nil {
		return nil
	}
	select {
	case <-restore.done:
		return restore.err
	case <-time.After(wakeWait):
		return errWaking
	case <-ctx.Done():
		return ctx.Err()
	}
}

// restoreLatest runs outside any request, so a caller that stops waiting does
// not cancel the restore the next caller needs.
func (d *Databases) restoreLatest(id string, restore *wake) {
	restore.err = d.RestoreAt(context.Background(), id, time.Time{}, filepath.Join(d.dir, id+".db"))
	d.mu.Lock()
	if restore.err == nil {
		d.lastUsed[id] = time.Now()
	}
	delete(d.waking, id)
	d.mu.Unlock()
	close(restore.done)
}

// RestoreAt writes database id as it was at the given time, or its latest
// state when at is zero, to a new file at path.
func (d *Databases) RestoreAt(ctx context.Context, id string, at time.Time, path string) error {
	client, err := litestream.NewReplicaClientFromURL(d.replicasURL + id)
	if err != nil {
		return err
	}
	options := litestream.NewRestoreOptions()
	options.OutputPath = path
	options.Timestamp = at
	err = litestream.NewReplicaWithClient(nil, client).Restore(ctx, options)
	// The replica answers the same for no database and for no copy that old.
	switch {
	case errors.Is(err, litestream.ErrTxNotAvailable) && at.IsZero():
		return errNotFound
	case errors.Is(err, litestream.ErrTxNotAvailable):
		return errNoCopyAtTime
	}
	return err
}
