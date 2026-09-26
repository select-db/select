package engine

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

// A guarded open must refuse a blocked address when it dials, not only in the
// pre-dial check, which a rebinding host passes.
func TestOpenGuardedDBRefusesABlockedAddressAtDial(t *testing.T) {
	_, err := postgresql.NewDialect().OpenGuardedDB("postgres://u:p@127.0.0.1:5432/x?sslmode=disable", guardedDial)
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("postgresql: err = %v, want not permitted", err)
	}

	db, err := mysql.NewDialect().OpenGuardedDB("u:p@tcp(127.0.0.1:3306)/x", guardedDial)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("mysql: err = %v, want not permitted", err)
	}

	if _, err := sqlite.NewDialect().OpenGuardedDB("file:/etc/passwd", guardedDial); err == nil {
		t.Fatal("sqlite: a guarded open of a file succeeded")
	}
}
