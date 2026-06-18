package db

import (
	"context"
	"database/sql"
	"sync"

	"selectDb/internal/db/generated"
)

// liveDB implements generated.DBTX by forwarding to the current global DB.
// This allows Queries to always use the active connection after a server switch,
// instead of holding a closed *sql.DB reference.
type liveDB struct{}

func (liveDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	conn := GetDB()
	if conn == nil {
		return nil, sql.ErrNoRows
	}
	return conn.ExecContext(ctx, query, args...)
}

func (liveDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	conn := GetDB()
	if conn == nil {
		return nil, sql.ErrNoRows
	}
	return conn.PrepareContext(ctx, query)
}

func (liveDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	conn := GetDB()
	if conn == nil {
		return nil, sql.ErrNoRows
	}
	return conn.QueryContext(ctx, query, args...)
}

var (
	errDBOnce sync.Once
	errDB     *sql.DB
)

func getErrDB() *sql.DB {
	errDBOnce.Do(func() {
		db, _ := sql.Open("sqlite3", ":memory:")
		_ = db.Close() // closed immediately; any query returns sql.ErrConnDone via *sql.Row
		errDB = db
	})
	return errDB
}

func (liveDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	conn := GetDB()
	if conn == nil {
		// Return a *sql.Row that will surface sql.ErrConnDone on Scan instead of panicking.
		return getErrDB().QueryRowContext(ctx, query, args...)
	}
	return conn.QueryRowContext(ctx, query, args...)
}

// Ensure liveDB implements generated.DBTX at compile time.
var _ generated.DBTX = liveDB{}

// NewLiveDB returns a DBTX that always uses the current global DB.
// Use this when constructing Queries so they follow server switches.
func NewLiveDB() generated.DBTX {
	return liveDB{}
}
