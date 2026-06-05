package db_client

import (
	"database/sql"
	"testing"

	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	_ "github.com/selectDb/dialect/sqlite" // blank import triggers package init(), registering SQLite dialect
	sqlitedriver "modernc.org/sqlite"
)

func init() {
	// Register pure-Go SQLite driver for tests (app registers it in production via app.go).
	sql.Register("sqlite3", &sqlitedriver.Driver{})
}

func TestOpenConn(t *testing.T) {
	dbc := &DbClient{}
	wsID := "ws1"
	dsn1, dsn2 := ":memory:", ":memory:2:"

	// Same (workspaceID, DSN) returns same instance
	db1, _ := dbc.GetOrOpenConn(wsID, "sqlite", dsn1, "folder1", nil)
	db2, _ := dbc.GetOrOpenConn(wsID, "sqlite", dsn1, "folder1", nil)
	if db1 != db2 {
		t.Fatal("expected cached DB instance")
	}

	// Different DSNs return different instances
	db3, _ := dbc.GetOrOpenConn(wsID, "sqlite", dsn2, "folder1", nil)
	if db3 == db1 {
		t.Fatal("expected different instances for different DSNs")
	}
}

func TestGetDialect(t *testing.T) {
	// Caching works
	d1 := engine.GetDialect("postgresql")
	d2 := engine.GetDialect("postgresql")
	if d1 != d2 {
		t.Fatal("expected cached dialect instance")
	}

	// Different types return different instances
	if d1 == engine.GetDialect("mysql") {
		t.Fatal("expected different instances for different types")
	}

	// Unsupported type returns nil
	if engine.GetDialect("unsupported") != nil {
		t.Fatal("expected nil for unsupported type")
	}

	// Type assertions
	if _, ok := d1.(*postgresql.Dialect); !ok {
		t.Fatal("expected *postgresql.Dialect")
	}
	if _, ok := engine.GetDialect("mysql").(*mysql.Dialect); !ok {
		t.Fatal("expected *mysql.Dialect")
	}

	// Concurrent access is safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			if engine.GetDialect("postgresql") == nil || engine.GetDialect("mysql") == nil {
				t.Error("expected non-nil dialects")
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Metadata caching is exercised in dialect/engine/metadata_test.go now
// that the cache lives there. App tests cover only the wiring layer.
