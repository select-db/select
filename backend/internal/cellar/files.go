package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Files holds the managed databases of one cellar, one SQLite file each.
type Files struct {
	dir string
	mu  sync.Mutex
	dbs map[string]*sql.DB
}

func NewFiles(dir string) *Files {
	return &Files{dir: dir, dbs: map[string]*sql.DB{}}
}

// Path is where the database id lives, and the key its schema is cached under.
func (f *Files) Path(id string) string {
	return filepath.Join(f.dir, id+".db")
}

// Open returns the database g names, with the caller's permissions, set up so
// every user statement runs under the isolation rules.
func (f *Files) Open(g Grant, perms core.CompiledPermissions) (engine.Conn, error) {
	if g.MaxBytes <= 0 {
		return engine.Conn{}, fmt.Errorf("grant for db %s has no size cap", g.DB)
	}
	db, err := f.pool(g.DB)
	if err != nil {
		return engine.Conn{}, err
	}
	return engine.Conn{
		DB:    db,
		Perms: perms,
		Prepare: func(c *sql.Conn, statement string) error {
			if err := CheckStatement(statement); err != nil {
				return err
			}
			return limit(c, g.MaxBytes)
		},
	}, nil
}

// pool opens the database id once and shares it across requests.
func (f *Files) pool(id string) (*sql.DB, error) {
	// The id becomes a file name.
	if _, err := uuid.Parse(id); err != nil {
		return nil, fmt.Errorf("db id %q is not a uuid", id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if db, ok := f.dbs[id]; ok {
		return db, nil
	}
	// mode=rw: a missing file is an error, never a new empty database.
	dsn := (&url.URL{Scheme: "file", Path: f.Path(id), RawQuery: "mode=rw&_defensive=1" +
		"&_busy_timeout=5000&_foreign_keys=1&_pragma=trusted_schema(0)"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// WAL lets readers run beside a writer. The file keeps it, and switching
	// needs the only connection, which this is.
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	f.dbs[id] = db
	return db, nil
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
