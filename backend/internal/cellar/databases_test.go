package cellar

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/litestream/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOpenDatabasesReplicatesWhatIsOnDisk(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	require.NoError(t, os.WriteFile(filepath.Join(cellar.dir, ".tmp-"+uuid.NewString()+".db"), nil, 0o600))
	require.NoError(t, CloseDatabases(context.Background()))

	require.NoError(t, OpenDatabases(cellar.dir, cellar.bucketDir))
	require.Len(t, databases.onDisk, 1, "WAL files and temporary copies are not databases")
	require.NotNil(t, databases.onDisk[id].replicating, "a write the last run had not sent reaches the bucket")
	require.Error(t, OpenDatabases(cellar.dir, cellar.bucketDir), "one cellar per process")
}

func TestCloseSendsTheLastWrites(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	cellar.exec(t, id, "INSERT INTO note VALUES ('c')")

	closed := databases
	require.NoError(t, CloseDatabases(context.Background()))
	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	require.NoError(t, closed.restoreAt(context.Background(), id, time.Time{}, restoredPath))
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
	databases.rest(context.Background(), time.Now().Add(restAfter))
	require.Nil(t, databases.onDisk[id].replicating)
	before := databases.onDisk[id].lastUsed

	path, err := databases.use(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cellar.dir, id+".db"), path)
	require.NotNil(t, databases.onDisk[id].replicating, "a resting database replicates again once used")
	require.True(t, databases.onDisk[id].lastUsed.After(before))
}

func TestUseMissingDatabase(t *testing.T) {
	newTestCellar(t)

	_, err := databases.use(context.Background(), uuid.NewString())
	require.ErrorIs(t, err, errNotFound)
	_, err = databases.use(context.Background(), "../escape")
	require.Error(t, err, "an id that is not a uuid")
}

func TestAddRefusesATakenID(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	tempPath := filepath.Join(cellar.dir, ".tmp-"+uuid.NewString()+".db")
	require.NoError(t, execTrusted(context.Background(), tempPath, "rwc", "PRAGMA journal_mode = WAL"))

	_, err := databases.add(id, tempPath)
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
	_, err := databases.use(context.Background(), onDiskID)
	require.NoError(t, err)

	for _, id := range []string{onDiskID, coldID} {
		require.NoError(t, databases.remove(context.Background(), id))
		require.NotContains(t, databases.onDisk, id)
		require.NoFileExists(t, filepath.Join(cellar.dir, id+".db"))
		require.NoDirExists(t, filepath.Join(cellar.dir, "."+id+".db-litestream"))
		require.NoDirExists(t, filepath.Join(cellar.bucketDir, "dbs", id), "a cold database leaves the bucket too")
	}
	require.NoError(t, databases.remove(context.Background(), uuid.NewString()), "removing an unknown id is not an error")
}

func TestBucketClientRefusesAnIDThatIsNotAUUID(t *testing.T) {
	newTestCellar(t)

	_, err := databases.bucketClient("../other")
	require.Error(t, err, "an id names a folder of the bucket")
}

func TestBucketClientKeepsTheURLQuery(t *testing.T) {
	bucket, err := url.Parse("s3://select-staging-cellar/dev-test?endpoint=https://s3.example.test&region=us-east-va")
	require.NoError(t, err)
	id := uuid.NewString()

	client, err := (&Databases{dir: t.TempDir(), bucket: *bucket}).bucketClient(id)
	require.NoError(t, err)
	s3Client, isS3 := client.(*s3.ReplicaClient)
	require.True(t, isS3)
	require.Equal(t, "select-staging-cellar", s3Client.Bucket)
	require.Equal(t, "dev-test/dbs/"+id, s3Client.Path, "the database's folder goes on the path, not after the query")
	require.Equal(t, "https://s3.example.test", s3Client.Endpoint)
	require.Equal(t, "us-east-va", s3Client.Region)
}

// TestAgainstTheBucket runs a database's whole life against CELLAR_TEST_BUCKET,
// an s3:// URL, under a folder of its own. Skipped when unset.
func TestAgainstTheBucket(t *testing.T) {
	setting := os.Getenv("CELLAR_TEST_BUCKET")
	if setting == "" {
		t.Skip("CELLAR_TEST_BUCKET is not set")
	}
	bucket, err := url.Parse(setting)
	require.NoError(t, err)
	bucket.Path = strings.TrimSuffix(bucket.Path, "/") + "/" + uuid.NewString()
	cellar := testCellar{dir: t.TempDir()}
	require.NoError(t, OpenDatabases(cellar.dir, bucket.String()))
	t.Cleanup(func() { _ = CloseDatabases(context.Background()) })
	ctx := context.Background()
	id := uuid.NewString()

	start := time.Now()
	_, err = createDatabase(ctx, id, CreateRequest{})
	require.NoError(t, err)
	cellar.exec(t, id, "CREATE TABLE note (body TEXT); INSERT INTO note VALUES ('a'), ('b')")
	t.Logf("create: %v", time.Since(start))

	start = time.Now()
	databases.rest(ctx, time.Now().Add(restAfter))
	require.Nil(t, databases.onDisk[id].replicating, "rested: the bucket took every write")
	t.Logf("rest (sync to the bucket): %v", time.Since(start))

	databases.evict(shortFor(1))
	require.NoFileExists(t, filepath.Join(cellar.dir, id+".db"))
	start = time.Now()
	_, err = databases.use(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 2, cellar.countNotes(t, id))
	t.Logf("wake (restore from the bucket): %v", time.Since(start))

	require.NoError(t, databases.remove(ctx, id))
	err = databases.restoreAt(ctx, id, time.Time{}, filepath.Join(t.TempDir(), "gone.db"))
	require.ErrorIs(t, err, errNoCopyAtTime, "remove deletes the bucket copy")
}
