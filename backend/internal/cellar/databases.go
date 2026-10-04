package cellar

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
	_ "github.com/benbjohnson/litestream/file"
	"github.com/benbjohnson/litestream/s3"
	"github.com/selectDb/dialect/engine/connect"
	"github.com/selectDb/dialect/engine/membudget"
	"golang.org/x/sync/singleflight"
)

// Databases are the managed databases on this cellar: their files on the
// cellar's disk and their copy in the bucket. Each one is
//   - replicating: used within restAfter, every write streams to the bucket;
//   - resting: on disk, and the bucket holds all of it;
//   - cold: in the bucket only, restored by its next use.
type Databases struct {
	dir         string
	bucket      url.URL
	store       *litestream.Store
	stopTidy    context.CancelFunc
	tidyStopped chan struct{}
	restores    singleflight.Group // one restore per cold database, shared by its callers

	mu     sync.Mutex
	onDisk map[string]*database // by id
}

// databases is the cellar this process runs, set by OpenDatabases.
var databases *Databases

// memoryBudget bounds the memory of the results being streamed, under the
// process's cap. OpenDatabases sizes it.
var memoryBudget *membudget.Budget

// database is one database file on the cellar's disk.
type database struct {
	id          string
	path        string
	lastUsed    time.Time
	replicating *litestream.DB // nil while resting
	// bucket is kept for the record's life: each client opens its own
	// connections, so a new one per rest and wake would redo the TLS handshake.
	bucket litestream.ReplicaClient
}

// bucketAccessKeyID and bucketSecretAccessKey are the keys of an s3:// bucket.
// Both stay empty for a directory bucket.
var bucketAccessKeyID, bucketSecretAccessKey string

// SetBucketKeys sets the keys the cellar signs its s3:// bucket requests with.
// It must run before OpenDatabases. Without it the S3 client falls back to the
// AWS environment variables, which a deployed cellar does not carry.
func SetBucketKeys(accessKeyID, secretAccessKey string) {
	bucketAccessKeyID, bucketSecretAccessKey = accessKeyID, secretAccessKey
}

// OpenDatabases starts the cellar's storage: it replicates the databases in
// dir to bucket, an s3:// URL or, without S3, a directory. The cellar's routes
// serve them until CloseDatabases.
func OpenDatabases(dir, bucket string) error {
	if databases != nil {
		return errors.New("cellar: databases already open")
	}
	if !litestream.IsURL(bucket) {
		absolute, err := filepath.Abs(bucket)
		if err != nil {
			return err
		}
		log.Printf("cellar: bucket is the directory %s: a lost machine loses it with the databases", absolute)
		bucket = "file://" + absolute
	}
	bucketURL, err := url.Parse(bucket)
	if err != nil {
		return err
	}
	memoryBudget = membudget.New(membudget.Size())
	log.Printf("cellar: memory budget %d MiB", membudget.Size()>>20)
	store := litestream.NewStore(nil, litestream.DefaultCompactionLevels)
	// One window for every plan: the backend refuses a point in time outside the plan's own.
	store.SnapshotInterval = 24 * time.Hour
	store.SnapshotRetention = 7 * 24 * time.Hour
	store.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if err := store.Open(context.Background()); err != nil {
		return err
	}
	ctx, stopTidy := context.WithCancel(context.Background())
	opened := &Databases{
		dir:         dir,
		bucket:      *bucketURL,
		store:       store,
		stopTidy:    stopTidy,
		tidyStopped: make(chan struct{}),
		onDisk:      map[string]*database{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
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
			return err
		}
		// Replicated once at start, so writes the last run had not sent reach
		// the bucket before the database can rest.
		database := &database{id: id, path: path, lastUsed: info.ModTime()}
		if err := opened.replicate(database); err != nil {
			return err
		}
		opened.onDisk[id] = database
	}
	go opened.tidyEvery(ctx)
	databases = opened
	return nil
}

// CloseDatabases stops the cellar's replication, after a last sync of every
// replicating database.
func CloseDatabases(ctx context.Context) error {
	if databases == nil {
		return nil
	}
	defer func() { databases = nil }()
	databases.stopTidy()
	<-databases.tidyStopped
	// Litestream's close syncs only a database it has opened; this opens it.
	var syncErrors []error
	for _, replicating := range databases.store.DBs() {
		syncErrors = append(syncErrors, replicating.SyncAndWait(ctx))
	}
	return errors.Join(append(syncErrors, databases.store.Close(ctx))...)
}

// use readies database id on this cellar for a statement: restored from the
// bucket if cold, replicating, its idle time reset. It returns the file's path.
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

// add moves a new database file into the cellar's directory, unless id is
// taken, and replicates it. It returns the file's path.
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

// remove deletes database id from the cellar's disk and from the bucket.
func (databases *Databases) remove(ctx context.Context, id string) error {
	path, err := databasePath(databases.dir, id)
	if err != nil {
		return err
	}
	client, err := databases.bucketClient(id)
	if err != nil {
		return err
	}
	// The backend stops sending statements for a database before it removes it,
	// so no wake races this.
	databases.mu.Lock()
	delete(databases.onDisk, id)
	connect.DeleteConnsByAddr(path)
	// A failed last sync does not matter: the bucket copy goes next.
	_ = databases.store.UnregisterDB(ctx, path)
	err = removeDatabaseFiles(path)
	databases.mu.Unlock()
	if err != nil {
		return err
	}
	return client.DeleteAll(ctx)
}

// replicate starts streaming database to the bucket, unless it already does.
// Callers hold databases.mu.
func (databases *Databases) replicate(database *database) error {
	if database.replicating != nil {
		return nil
	}
	if database.bucket == nil {
		client, err := databases.bucketClient(database.id)
		if err != nil {
			return err
		}
		database.bucket = client
	}
	replicating := litestream.NewDB(database.path)
	replicating.Replica = litestream.NewReplicaWithClient(replicating, database.bucket)
	if err := databases.store.RegisterDB(replicating); err != nil {
		return err
	}
	database.replicating = replicating
	return nil
}

// bucketClient reaches database id's copy at dbs/{id}/ in the bucket. It checks
// id as databasePath does: the id names a folder there.
func (databases *Databases) bucketClient(id string) (litestream.ReplicaClient, error) {
	if _, err := databasePath(databases.dir, id); err != nil {
		return nil, err
	}
	// Joined on the path: an s3:// URL carries its endpoint and region in the query.
	location := databases.bucket
	location.Path = strings.TrimSuffix(location.Path, "/") + "/dbs/" + id
	client, err := litestream.NewReplicaClientFromURL(location.String())
	if err != nil {
		return nil, err
	}
	if s3Client, isS3 := client.(*s3.ReplicaClient); isS3 && bucketAccessKeyID != "" {
		s3Client.AccessKeyID, s3Client.SecretAccessKey = bucketAccessKeyID, bucketSecretAccessKey
	}
	return client, nil
}
