package query

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// failed runs one statement and reports whether it failed.
func failed(conn Conn, stmt string) bool {
	return len(Execute(context.Background(), conn, DBInstance{ID: "x"}, stmt, Options{}).Errors) > 0
}

func TestStreamRunsPrepareOnTheStatementConnection(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t(id INTEGER)"); err != nil {
		t.Fatal(err)
	}

	readOnly := func(c *sql.Conn, _ string) error {
		_, err := c.ExecContext(context.Background(), "PRAGMA query_only = 1")
		return err
	}
	if !failed(Conn{DB: db, Prepare: readOnly}, "INSERT INTO t VALUES (1)") {
		t.Fatal("write ran on a connection Prepare made read-only")
	}

	refuse := func(_ *sql.Conn, stmt string) error { return errors.New("refused " + stmt) }
	if !failed(Conn{DB: db, Prepare: refuse}, "INSERT INTO t VALUES (1)") {
		t.Fatal("statement ran after Prepare failed")
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows = %d, err = %v; want 0 rows", n, err)
	}
}
