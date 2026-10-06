package cellar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/litestream"
)

func TestStatsCountTheStatesTheCountersAndTheBytes(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	databases := &Databases{onDisk: map[string]*database{
		"a": {id: "a", path: write("a.db", 1000), replicating: &litestream.DB{}},
		"b": {id: "b", path: write("b.db", 500)},
		"c": {id: "c", path: write("c.db", 250)},
	}}
	databases.wakes.Add(3)
	databases.evictions.Add(1)

	// the size is read once and reused, so reset it as the first read of a test
	sizeMu.Lock()
	sizeAt = time.Time{}
	sizeMu.Unlock()
	now := time.Now()

	report := databases.stats(now)
	if report.Replicating != 1 || report.Resting != 2 || report.Wakes != 3 || report.Evictions != 1 || report.Bytes != 1750 {
		t.Fatalf("%+v", report)
	}

	// a file that grows is not seen until the size is read again
	if err := os.WriteFile(filepath.Join(dir, "b.db"), make([]byte, 5000), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := databases.stats(now.Add(10 * time.Second)).Bytes; got != 1750 {
		t.Fatalf("within the ttl the size is reused: %d", got)
	}
	if got := databases.stats(now.Add(31 * time.Second)).Bytes; got != 6250 {
		t.Fatalf("after the ttl it is read again: %d", got)
	}
}

func TestStatsOfACellarThatIsNotOpen(t *testing.T) {
	saved := databases
	databases = nil
	defer func() { databases = saved }()
	if report := Stats().(StatsReport); report != (StatsReport{}) {
		t.Fatalf("%+v", report)
	}
}
