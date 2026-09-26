package cellar

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine"
	"modernc.org/sqlite"
)

func init() {
	// The engine's sqlite dialect opens "sqlite3", which the desktop app
	// registers too.
	sql.Register("sqlite3", sqliteDriver{&sqlite.Driver{}})
}

// sqliteDriver is modernc's driver, whose Query runs every statement: a failed
// one must not be rerun as an exec, or a script's earlier writes run twice.
type sqliteDriver struct{ *sqlite.Driver }

func (sqliteDriver) QueryRunsAll() {}

// dbType is the engine dialect of every database a cellar holds.
const dbType = "sqlite"

// Open returns the grant's datasource, the file named after its id in dir, set
// up so every statement runs under the isolation rules.
func Open(dir string, grant Grant) (engine.Conn, error) {
	if grant.MaxBytes <= 0 {
		return engine.Conn{}, fmt.Errorf("grant for datasource %s has no size cap", grant.DatasourceID)
	}

	if _, err := uuid.Parse(grant.DatasourceID); err != nil {
		return engine.Conn{}, fmt.Errorf("datasource id %q is not a uuid", grant.DatasourceID)
	}

	// mode=rw: a missing file is an error, never a new empty database. WAL lets
	// readers run beside a writer.
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(dir, grant.DatasourceID+".db"), RawQuery: "mode=rw&_defensive=1" +
		"&_busy_timeout=5000&_foreign_keys=1&_pragma=trusted_schema(0)&_pragma=journal_mode(WAL)"}).String()
	db, err := engine.GetOrOpenTrusted(grant.WorkspaceID, dbType, dsn)

	if err != nil {
		return engine.Conn{}, err
	}
	return engine.Conn{
		DB: db,
		Prepare: func(c *sql.Conn, statement string) error {
			return isolate(c, statement, grant.MaxBytes)
		},
	}, nil
}
