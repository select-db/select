package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
		_, err = databases.use(ctx, id)
		require.NoError(t, err)
		wake := time.Since(start)

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
	conn, err := sql.Open("sqlite", path)
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
	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(statement)
	require.NoError(t, err)
}
