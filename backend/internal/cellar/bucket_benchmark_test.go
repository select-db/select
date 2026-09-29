package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/superfly/ltx"
)

// TestBucketBenchmark times the bucket path for databases of CELLAR_BENCH_MB
// sizes (default 10,100,250) against CELLAR_TEST_BUCKET. Temporary: remove once
// the numbers are in litestream-spike.md.
func TestBucketBenchmark(t *testing.T) {
	setting := os.Getenv("CELLAR_TEST_BUCKET")
	if setting == "" {
		t.Skip("CELLAR_TEST_BUCKET is not set")
	}
	sizes := os.Getenv("CELLAR_BENCH_MB")
	if sizes == "" {
		sizes = "10,100,250"
	}
	bucket, err := url.Parse(setting)
	require.NoError(t, err)
	bucket.Path = strings.TrimSuffix(bucket.Path, "/") + "/bench-" + uuid.NewString()
	dir := t.TempDir()
	require.NoError(t, OpenDatabases(dir, bucket.String()))
	t.Cleanup(func() { _ = CloseDatabases(context.Background()) })
	ctx := context.Background()

	report := []string{"| size | first upload | wake (restore latest) | restore at a time | remove |", "| --- | --- | --- | --- | --- |"}
	for _, size := range strings.Split(sizes, ",") {
		megabytes, err := strconv.Atoi(strings.TrimSpace(size))
		require.NoError(t, err)
		id := uuid.NewString()
		_, err = createDatabase(ctx, id, CreateRequest{})
		require.NoError(t, err)
		path := filepath.Join(dir, id+".db")
		fillWithSales(t, path, megabytes)
		fileBytes, err := databaseSize(path)
		require.NoError(t, err)
		fileMB := float64(fileBytes) / (1 << 20)

		// Rest is the first full upload: the monitor has had no time to send it.
		start := time.Now()
		databases.rest(ctx, time.Now().Add(restAfter))
		upload := time.Since(start)
		require.Nil(t, databases.onDisk[id].replicating)

		mark := time.Now()
		time.Sleep(1100 * time.Millisecond)
		_, err = databases.use(ctx, id)
		require.NoError(t, err)
		execOn(t, path, "INSERT INTO sale (id, note) VALUES (-1, 'after the mark')")
		databases.rest(ctx, time.Now().Add(restAfter))

		databases.evict(shortFor(1))
		require.NoFileExists(t, path)
		start = time.Now()
		// use answers errWaking after wakeWait while the restore goes on, so
		// time what a client sees: retry until the database is on disk.
		require.Eventually(t, func() bool {
			_, err := databases.use(ctx, id)
			return err == nil
		}, 20*time.Minute, time.Second)
		wake := time.Since(start)

		traceRestore(t, id)

		start = time.Now()
		require.NoError(t, databases.restoreAt(ctx, id, mark, filepath.Join(t.TempDir(), "at.db")))
		restoreAt := time.Since(start)

		start = time.Now()
		require.NoError(t, databases.remove(ctx, id))
		remove := time.Since(start)

		report = append(report, fmt.Sprintf("| %.0f MB | %.1fs (%.0f MB/s) | %.1fs (%.0f MB/s) | %.1fs | %.1fs |",
			fileMB, upload.Seconds(), fileMB/upload.Seconds(), wake.Seconds(), fileMB/wake.Seconds(), restoreAt.Seconds(), remove.Seconds()))
	}
	t.Log("\n" + strings.Join(report, "\n"))
}

// fillWithSales writes wordy rows, compressible like business data, until the
// file reaches megabytes.
func fillWithSales(t *testing.T, path string, megabytes int) {
	t.Helper()
	// The cellar replicates the same file while this writes, so wait it out
	// rather than failing on SQLITE_BUSY.
	conn, err := sql.Open("sqlite", "file:"+path+"?_busy_timeout=30000")
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec("CREATE TABLE sale (id INTEGER PRIMARY KEY, note TEXT)")
	require.NoError(t, err)
	for next := 0; ; next += 100_000 {
		size, err := databaseSize(path)
		require.NoError(t, err)
		if size >= int64(megabytes)<<20 {
			return
		}
		_, err = conn.Exec(`WITH RECURSIVE counter(n) AS (SELECT ? UNION ALL SELECT n + 1 FROM counter WHERE n < ? + 99999)
			INSERT INTO sale SELECT n, printf('order %d for customer %d shipped to %s, %d units of %s',
				n, n % 9973, substr('Paris Lyon Lille Nantes Boston Denver Austin Seattle', 1 + n % 40, 12),
				n % 17, substr('blue chair red table green lamp oak desk steel shelf', 1 + n % 35, 16))
			FROM counter`, next, next)
		require.NoError(t, err)
	}
}

func execOn(t *testing.T, path, statement string) {
	t.Helper()
	conn, err := sql.Open("sqlite", "file:"+path+"?_busy_timeout=30000")
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(statement)
	require.NoError(t, err)
}

// traceRestore restores database id again through a client that logs each
// bucket call, to show where a wake spends its time.
func traceRestore(t *testing.T, id string) {
	t.Helper()
	client, err := databases.bucketClient(id)
	require.NoError(t, err)
	traced := &tracedClient{ReplicaClient: client, t: t, start: time.Now()}
	options := litestream.NewRestoreOptions()
	options.OutputPath = filepath.Join(t.TempDir(), "traced.db")
	require.NoError(t, litestream.NewReplicaWithClient(nil, traced).Restore(context.Background(), options))
	t.Logf("traced restore of %s: %v in all", id, time.Since(traced.start))
}

type tracedClient struct {
	litestream.ReplicaClient
	t     *testing.T
	start time.Time
}

func (client *tracedClient) LTXFiles(ctx context.Context, level int, seek ltx.TXID, useMetadata bool) (ltx.FileIterator, error) {
	start := time.Now()
	files, err := client.ReplicaClient.LTXFiles(ctx, level, seek, useMetadata)
	client.t.Logf("  %6dms list level %d took %v", time.Since(client.start).Milliseconds(), level, time.Since(start))
	return files, err
}

func (client *tracedClient) OpenLTXFile(ctx context.Context, level int, minTXID, maxTXID ltx.TXID, offset, size int64) (io.ReadCloser, error) {
	start := time.Now()
	reader, err := client.ReplicaClient.OpenLTXFile(ctx, level, minTXID, maxTXID, offset, size)
	client.t.Logf("  %6dms open level %d file %s-%s took %v", time.Since(client.start).Milliseconds(), level, minTXID, maxTXID, time.Since(start))
	if err != nil {
		return nil, err
	}
	return &tracedReader{ReadCloser: reader, client: client, name: fmt.Sprintf("level %d file %s-%s", level, minTXID, maxTXID), start: time.Now()}, nil
}

type tracedReader struct {
	io.ReadCloser
	client *tracedClient
	name   string
	start  time.Time
	bytes  int64
}

func (reader *tracedReader) Read(buffer []byte) (int, error) {
	count, err := reader.ReadCloser.Read(buffer)
	reader.bytes += int64(count)
	return count, err
}

func (reader *tracedReader) Close() error {
	elapsed := time.Since(reader.start)
	reader.client.t.Logf("  %6dms read %s: %.1f MB in %v (%.0f MB/s)", time.Since(reader.client.start).Milliseconds(), reader.name,
		float64(reader.bytes)/(1<<20), elapsed, float64(reader.bytes)/(1<<20)/elapsed.Seconds())
	return reader.ReadCloser.Close()
}
