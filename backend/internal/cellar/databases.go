package cellar

import (
	"context"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
	_ "github.com/benbjohnson/litestream/file"
	_ "github.com/benbjohnson/litestream/s3"
	"github.com/selectDb/dialect/engine/connect"
)

// Databases are the database files of a cellar and their replica. Each one is
//   - replicating: used within restAfter, every write streams to the replica;
//   - resting: on disk, and the replica holds all of it;
//   - cold: in the replica only, restored by its next Use.
type Databases struct {
	dir         string
	replicasURL string // a database's replica is replicasURL + its id
	store       *litestream.Store
	stopTidy    context.CancelFunc

	mu       sync.Mutex
	lastUsed map[string]time.Time // every database on disk, by id
	waking   map[string]*wake
}

// OpenDatabases replicates the databases in dir to replica, a directory or a
// Litestream replica URL such as s3://bucket/path.
func OpenDatabases(dir, replica string) (*Databases, error) {
	if !litestream.IsURL(replica) {
		absolute, err := filepath.Abs(replica)
		if err != nil {
			return nil, err
		}
		log.Printf("cellar: replica in the directory %s: a lost machine loses it with the databases", absolute)
		replica = "file://" + absolute
	}
	store := litestream.NewStore(nil, litestream.DefaultCompactionLevels)
	// One window for every plan: the backend refuses a point in time outside the plan's own.
	store.SnapshotInterval = 24 * time.Hour
	store.SnapshotRetention = 7 * 24 * time.Hour
	store.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if err := store.Open(context.Background()); err != nil {
		return nil, err
	}
	ctx, stopTidy := context.WithCancel(context.Background())
	databases := &Databases{
		dir:         dir,
		replicasURL: replica + "/dbs/",
		store:       store,
		stopTidy:    stopTidy,
		lastUsed:    map[string]time.Time{},
		waking:      map[string]*wake{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		id, isDatabase := databaseID(entry.Name())
		info, err := entry.Info()
		if !isDatabase || err != nil {
			continue
		}
		// Replicated once at start, so writes the last run had not sent reach
		// the replica before the database can rest.
		if err := databases.replicate(id); err != nil {
			return nil, err
		}
		databases.lastUsed[id] = info.ModTime()
	}
	go databases.tidyEvery(ctx, time.Minute)
	return databases, nil
}

// Close stops replicating, after a last sync of every replicating database.
func (d *Databases) Close(ctx context.Context) error {
	d.stopTidy()
	return d.store.Close(ctx)
}

// Use readies database id for a statement: restored if cold, replicating, its
// idle time reset. It returns the file's path.
func (d *Databases) Use(ctx context.Context, id string) (string, error) {
	path, err := databasePath(d.dir, id)
	if err != nil {
		return "", err
	}
	if err := d.wake(ctx, id); err != nil {
		return "", err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, onDisk := d.lastUsed[id]; !onDisk {
		// Evicted between its wake and now.
		return "", errWaking
	}
	if err := d.replicate(id); err != nil {
		return "", err
	}
	d.lastUsed[id] = time.Now()
	return path, nil
}

// add moves a new database file into place and replicates it.
func (d *Databases) add(id, tempPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := os.Rename(tempPath, filepath.Join(d.dir, id+".db")); err != nil {
		return err
	}
	d.lastUsed[id] = time.Now()
	return d.replicate(id)
}

// Delete removes database id from disk and from the replica.
func (d *Databases) Delete(ctx context.Context, id string) error {
	path, err := databasePath(d.dir, id)
	if err != nil {
		return err
	}
	client, err := litestream.NewReplicaClientFromURL(d.replicasURL + id)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	connect.DeleteConnsByAddr(path)
	// A failed last sync does not matter: the replica goes next.
	_ = d.store.UnregisterDB(ctx, path)
	delete(d.lastUsed, id)
	if err := removeDatabaseFiles(d.dir, id); err != nil {
		return err
	}
	return client.DeleteAll(ctx)
}

// replicate starts streaming database id to its replica, unless it already
// does. Callers hold d.mu.
func (d *Databases) replicate(id string) error {
	path := filepath.Join(d.dir, id+".db")
	if d.store.FindDB(path) != nil {
		return nil
	}
	client, err := litestream.NewReplicaClientFromURL(d.replicasURL + id)
	if err != nil {
		return err
	}
	db := litestream.NewDB(path)
	db.Replica = litestream.NewReplicaWithClient(db, client)
	return d.store.RegisterDB(db)
}
