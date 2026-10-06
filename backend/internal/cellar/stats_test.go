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

	sizes.Delete("sizes")
	now := time.Now()

	report := databases.stats(now)
	if report.Replicating != 1 || report.Resting != 2 || report.Wakes != 3 || report.Evictions != 1 || report.Bytes != 1750 {
		t.Fatalf("%+v", report)
	}
	if len(report.Largest) != 3 || report.Largest[0].ID != "a" || report.Largest[0].State != "replicating" || report.Largest[2].ID != "c" {
		t.Fatalf("the largest first: %+v", report.Largest)
	}
	if report.NeverSynced != 1 {
		t.Fatalf("a database that has not synced is counted: %+v", report)
	}

	// a file that grows is not seen while the size is reused
	if err := os.WriteFile(filepath.Join(dir, "b.db"), make([]byte, 5000), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := databases.stats(now).Bytes; got != 1750 {
		t.Fatalf("within the ttl the size is reused: %d", got)
	}
	sizes.Delete("sizes")
	if got := databases.stats(now).Bytes; got != 6250 {
		t.Fatalf("read again, it is current: %d", got)
	}
}

func TestStatsKeepOnlyTheLargestDatabases(t *testing.T) {
	dir := t.TempDir()
	databases := &Databases{onDisk: map[string]*database{}}
	for i := 0; i < largestCount+5; i++ {
		path := filepath.Join(dir, "d"+string(rune('a'+i))+".db")
		if err := os.WriteFile(path, make([]byte, 100+i), 0o600); err != nil {
			t.Fatal(err)
		}
		id := "d" + string(rune('a'+i))
		databases.onDisk[id] = &database{id: id, path: path}
	}
	sizes.Delete("sizes")

	report := databases.stats(time.Now())

	if len(report.Largest) != largestCount || report.Largest[0].Bytes != int64(100+largestCount+4) {
		t.Fatalf("%d kept, first %+v", len(report.Largest), report.Largest[0])
	}
}

func TestStatsOfACellarThatIsNotOpen(t *testing.T) {
	saved := databases
	databases = nil
	defer func() { databases = saved }()
	if report := Stats().(StatsReport); report.Replicating+report.Resting != 0 || report.Bytes != 0 || report.Largest != nil {
		t.Fatalf("%+v", report)
	}
}
