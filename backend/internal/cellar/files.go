package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func init() {
	// The engine's sqlite dialect opens "sqlite3", which the desktop app
	// registers the same way.
	sql.Register("sqlite3", &sqlite.Driver{})
}

// Files holds the managed databases of one cellar, one SQLite file each.
type Files struct {
	dir string
}

func NewFiles(dir string) *Files {
	return &Files{dir: dir}
}

// Path is where the database id lives, and the key its schema is cached under.
func (f *Files) Path(id string) string {
	return filepath.Join(f.dir, id+".db")
}

// Open returns the grant's datasource, set up so every statement runs under
// the isolation rules.
func (f *Files) Open(grant Grant) (engine.Conn, error) {
	if grant.MaxBytes <= 0 {
		return engine.Conn{}, fmt.Errorf("grant for datasource %s has no size cap", grant.DatasourceID)
	}

	if _, err := uuid.Parse(grant.DatasourceID); err != nil {
		return engine.Conn{}, fmt.Errorf("datasource id %q is not a uuid", grant.DatasourceID)
	}

	// mode=rw: a missing file is an error, never a new empty database. WAL lets
	// readers run beside a writer.
	dsn := (&url.URL{Scheme: "file", Path: f.Path(grant.DatasourceID), RawQuery: "mode=rw&_defensive=1" +
		"&_busy_timeout=5000&_foreign_keys=1&_pragma=trusted_schema(0)&_pragma=journal_mode(WAL)"}).String()
	db, err := engine.GetOrOpenTrusted(grant.WorkspaceID, dbType, dsn)

	if err != nil {
		return engine.Conn{}, err
	}
	return engine.Conn{
		DB: db,
		Prepare: func(c *sql.Conn, statement string) error {
			if err := CheckStatement(statement); err != nil {
				return err
			}
			return limit(c, grant.MaxBytes)
		},
	}, nil
}

// limit applies the settings SQLite keeps per connection, before each user
// statement: no ATTACH (which VACUUM INTO also needs), and the size cap.
func limit(c *sql.Conn, maxBytes int64) error {
	if _, err := sqlite.Limit(c, sqlite3.SQLITE_LIMIT_ATTACHED, 0); err != nil {
		return err
	}

	ctx := context.Background()
	var pageSize int64
	if err := c.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}

	_, err := c.ExecContext(ctx, fmt.Sprintf("PRAGMA max_page_count = %d", max(maxBytes/pageSize, 1)))
	return err
}
