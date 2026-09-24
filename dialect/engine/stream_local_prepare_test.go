package engine

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// outcome runs one statement through StreamLocal and returns the recorded events.
func outcome(t *testing.T, conn Conn, stmt string) []string {
	t.Helper()
	listener := &recordingListener{}
	StreamLocal(context.Background(), conn, DBInstance{ID: "x"}, stmt, Options{},
		NewStreamingSink(NewStreamingResult("prepare"), listener))
	listener.mu.Lock()
	defer listener.mu.Unlock()
	return listener.events
}

func TestStreamLocalRunsPrepareOnTheStatementConnection(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t(id INTEGER)"); err != nil {
		t.Fatal(err)
	}

	readOnly := func(c *sql.Conn) error {
		_, err := c.ExecContext(context.Background(), "PRAGMA query_only = 1")
		return err
	}
	if got := outcome(t, Conn{DB: db, Prepare: readOnly}, "INSERT INTO t VALUES (1)"); got[len(got)-1] != "error" {
		t.Fatalf("write ran on a connection Prepare made read-only: %v", got)
	}

	refuse := func(*sql.Conn) error { return errors.New("refused") }
	if got := outcome(t, Conn{DB: db, Prepare: refuse}, "INSERT INTO t VALUES (1)"); got[len(got)-1] != "error" {
		t.Fatalf("statement ran after Prepare failed: %v", got)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows = %d, err = %v; want 0 rows", n, err)
	}
}
