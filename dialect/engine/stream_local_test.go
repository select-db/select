package engine

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type recordingListener struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingListener) record(s string) {
	r.mu.Lock()
	r.events = append(r.events, s)
	r.mu.Unlock()
}

func (r *recordingListener) OnStart(_ []string, _ []ColumnEditMeta) { r.record("started") }
func (r *recordingListener) OnExecuted(_ int64)                      { r.record("executed") }
func (r *recordingListener) OnProgress(_ int64)                      { r.record("progress") }
func (r *recordingListener) OnDone(_, _, _ int64)                    { r.record("done") }
func (r *recordingListener) OnError(_ string, _ *int)                { r.record("error") }

func TestStreamLocalEmitsStartedThenExecuted(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t VALUES (1),(2),(3)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	listener := &recordingListener{}
	sink := NewStreamingSink(NewStreamingResult("test-exec"), listener)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	StreamLocal(ctx, Conn{DB: db}, DBInstance{ID: "x"}, "SELECT id FROM t ORDER BY id", Options{}, sink)

	listener.mu.Lock()
	defer listener.mu.Unlock()

	wantPrefix := []string{"started", "executed"}
	if len(listener.events) < 2 {
		t.Fatalf("expected at least started+executed, got %v", listener.events)
	}
	for i, want := range wantPrefix {
		if listener.events[i] != want {
			t.Fatalf("event[%d]: want %q, got %q (full: %v)", i, want, listener.events[i], listener.events)
		}
	}

	last := listener.events[len(listener.events)-1]
	if last != "done" {
		t.Fatalf("last event: want done, got %q (full: %v)", last, listener.events)
	}
}
