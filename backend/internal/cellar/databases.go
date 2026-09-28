package cellar

import (
	"context"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
	_ "github.com/benbjohnson/litestream/file"
	_ "github.com/benbjohnson/litestream/s3"
	"github.com/selectDb/dialect/engine/connect"
	"golang.org/x/sync/singleflight"
)

// Databases are the database files of a cellar and their replica. Each one is
//   - replicating: used within restAfter, every write streams to the replica;
//   - resting: on disk, and the replica holds all of it;
//   - cold: in the replica only, restored by its next use.
type Databases struct {
	dir         string
	replicasURL string // a database's replica is replicasURL + its id
	store       *litestream.Store
	stopTidy    context.CancelFunc
	tidyStopped chan struct{}
	restores    singleflight.Group // one restore per cold database, shared by its callers

	mu     sync.Mutex
	onDisk map[string]*database // by id
}

// database is one database file on disk.
type database struct {
	id          string
	path        string
	lastUsed    time.Time
	replicating *litestream.DB // nil while resting
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
	opened := &Databases{
		dir:         dir,
		replicasURL: replica + "/dbs/",
		store:       store,
		stopTidy:    stopTidy,
		tidyStopped: make(chan struct{}),
		onDisk:      map[string]*database{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		// Only <uuid>.db is a database, not its WAL files or a temporary copy.
		id, isDatabase := strings.CutSuffix(entry.Name(), ".db")
		path, err := databasePath(dir, id)
		if !isDatabase || err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		// Replicated once at start, so writes the last run had not sent reach
		// the replica before the database can rest.
		database := &database{id: id, path: path, lastUsed: info.ModTime()}
		if err := opened.replicate(database); err != nil {
			return nil, err
		}
		opened.onDisk[id] = database
	}
	go opened.tidyEvery(ctx)
	return opened, nil
}

// Close stops replicating, after a last sync of every replicating database.
func (databases *Databases) Close(ctx context.Context) error {
	databases.stopTidy()
	<-databases.tidyStopped
	return databases.store.Close(ctx)
}

// use readies database id for a statement: restored if cold, replicating, its
// idle time reset. It returns the file's path.
func (databases *Databases) use(ctx context.Context, id string) (string, error) {
	path, err := databasePath(databases.dir, id)
	if err != nil {
		return "", err
	}
	if err := databases.wake(ctx, id, path); err != nil {
		return "", err
	}
	databases.mu.Lock()
	defer databases.mu.Unlock()
	database, onDisk := databases.onDisk[id]
	if !onDisk {
		// Evicted between its wake and now.
		return "", errWaking
	}
	if err := databases.replicate(database); err != nil {
		return "", err
	}
	database.lastUsed = time.Now()
	return path, nil
}

// add moves a new database file into place, unless id is taken, and
// replicates it. It returns the file's path.
func (databases *Databases) add(id, tempPath string) (string, error) {
	path, err := databasePath(databases.dir, id)
	if err != nil {
		return "", err
	}
	databases.mu.Lock()
	defer databases.mu.Unlock()
	if _, onDisk := databases.onDisk[id]; onDisk {
		return "", errAlreadyExists
	}
	if err := os.Rename(tempPath, path); err != nil {
		return "", err
	}
	database := &database{id: id, path: path, lastUsed: time.Now()}
	if err := databases.replicate(database); err != nil {
		return "", err
	}
	databases.onDisk[id] = database
	return path, nil
}

// remove deletes database id from disk and from the replica.
func (databases *Databases) remove(ctx context.Context, id string) error {
	path, err := databasePath(databases.dir, id)
	if err != nil {
		return err
	}
	client, err := databases.replicaClient(id)
	if err != nil {
		return err
	}
	// The backend stops sending statements for a database before it removes it,
	// so no wake races this.
	databases.mu.Lock()
	delete(databases.onDisk, id)
	connect.DeleteConnsByAddr(path)
	// A failed last sync does not matter: the replica goes next.
	_ = databases.store.UnregisterDB(ctx, path)
	err = removeDatabaseFiles(path)
	databases.mu.Unlock()
	if err != nil {
		return err
	}
	return client.DeleteAll(ctx)
}

// replicate starts streaming database to its replica, unless it already does.
// Callers hold databases.mu.
func (databases *Databases) replicate(database *database) error {
	if database.replicating != nil {
		return nil
	}
	client, err := databases.replicaClient(database.id)
	if err != nil {
		return err
	}
	replicating := litestream.NewDB(database.path)
	replicating.Replica = litestream.NewReplicaWithClient(replicating, client)
	if err := databases.store.RegisterDB(replicating); err != nil {
		return err
	}
	database.replicating = replicating
	return nil
}

// replicaClient checks id as databasePath does: it names a folder of the bucket.
func (databases *Databases) replicaClient(id string) (litestream.ReplicaClient, error) {
	if _, err := databasePath(databases.dir, id); err != nil {
		return nil, err
	}
	return litestream.NewReplicaClientFromURL(databases.replicasURL + id)
}
