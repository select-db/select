package engine

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// countingSink records row count and whether OnTruncated fired. Used to
// verify StreamLocal honors Options.MaxRows.
type countingSink struct {
	mu        sync.Mutex
	rows      int
	truncated bool
	done      bool
	errored   bool
}

func (s *countingSink) OnColumns(_ []string) error { return nil }
func (s *countingSink) OnRow(_ []any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows++
	return nil
}
func (s *countingSink) OnDone(_, _, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
	return nil
}
func (s *countingSink) OnError(_ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errored = true
}
func (s *countingSink) OnTruncated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.truncated = true
}

func newDBWithRows(t *testing.T, n int) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 1; i <= n; i++ {
		if _, err := db.Exec("INSERT INTO t VALUES (?)", i); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return db
}

func TestStreamLocal_MaxRowsCapsScan(t *testing.T) {
	db := newDBWithRows(t, 100)
	defer db.Close()

	sink := &countingSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	StreamLocal(ctx, Conn{DB: db}, DBInstance{ID: "x"},
		"SELECT id FROM t ORDER BY id",
		Options{MaxRows: 10}, sink)

	if sink.rows != 10 {
		t.Fatalf("want 10 rows emitted, got %d", sink.rows)
	}
	if !sink.truncated {
		t.Fatalf("expected OnTruncated to fire when more rows are available")
	}
	if !sink.done {
		t.Fatalf("expected OnDone to fire after a successful cap")
	}
	if sink.errored {
		t.Fatalf("did not expect OnError on a clean truncation")
	}
}

func TestStreamLocal_MaxRowsSkipsWhenUnderCap(t *testing.T) {
	db := newDBWithRows(t, 5)
	defer db.Close()

	sink := &countingSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	StreamLocal(ctx, Conn{DB: db}, DBInstance{ID: "x"},
		"SELECT id FROM t ORDER BY id",
		Options{MaxRows: 50}, sink)

	if sink.rows != 5 {
		t.Fatalf("want 5 rows, got %d", sink.rows)
	}
	if sink.truncated {
		t.Fatalf("did not expect OnTruncated when result fits under the cap")
	}
}

func TestStreamLocal_MaxRowsZeroIsUnbounded(t *testing.T) {
	db := newDBWithRows(t, 30)
	defer db.Close()

	sink := &countingSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	StreamLocal(ctx, Conn{DB: db}, DBInstance{ID: "x"},
		"SELECT id FROM t ORDER BY id",
		Options{ /* MaxRows: 0 */ }, sink)

	if sink.rows != 30 {
		t.Fatalf("want 30 rows (unbounded), got %d", sink.rows)
	}
	if sink.truncated {
		t.Fatalf("did not expect OnTruncated when MaxRows is unset")
	}
}
