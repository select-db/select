// Package sample builds the workspace a person gets on day one: a small
// database with data in it, the queries that read it, and the dotfiles that
// govern them.
//
// It has one definition because it has two callers. A new workspace is seeded
// with it, and every screenshot on the website is taken of it, so the guide
// describes the reader's own screen rather than a fixture that resembles it.
// Anything that belongs to a test account and not to a workspace -- users,
// roles, a git history with work in progress -- is layered on top by
// internal/cmd/e2eseed and does not live here.
package sample

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/graph"
	"selectDb/internal/utils"
)

// WarehouseID is the sample database's id, shared by its db.config.json, the
// metadata sidecar that associates a query file with it, and any permission
// granted on it.
const WarehouseID = "sample-warehouse"

// WarehouseName is what the database is called in the tree.
const WarehouseName = "warehouse"

// Files is the sample workspace's queries, by name. Exported because the e2e
// fixture rewrites two of them to build a git history.
var Files = map[string]string{
	"weekly_revenue.sql": WeeklyRevenueSQL,
	"top_customers.sql":  TopCustomersSQL,
	"cohorts.sql":        CohortsSQL,
}

// database/sql resolves a driver by a name in a package-global registry, and
// sql.Register panics on a duplicate. The app registers "sqlite3" at startup
// and the e2e fixture registers it too, so this package can neither rely on
// that having happened nor claim the name itself. Its own name, registered
// once, makes writing the sample self-contained wherever it is called from.
const driverName = "sqlite3-sample"

var registerDriver sync.Once

func openSampleDB(path string) (*sql.DB, error) {
	registerDriver.Do(func() { sql.Register(driverName, &sqlite.Driver{}) })
	return sql.Open(driverName, path)
}

// Write seeds workspaceID's root with the sample workspace: the shipped .lint
// and .gitignore, a warehouse database with data in it, the .env that points
// at it, and the queries that read it.
//
// Every write skips a file that already exists, so calling this on a workspace
// somebody has edited changes nothing of theirs -- including the database
// itself, which is never truncated once it holds rows.
func Write(workspaceID string) error {
	root, err := graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		return err
	}

	if err := graph.SeedDefaultFiles(root); err != nil {
		return fmt.Errorf("workspace defaults: %w", err)
	}
	if err := writeQueries(root); err != nil {
		return err
	}
	return writeDatabase(root)
}

// writeQueries puts the example queries in the workspace root.
func writeQueries(root string) error {
	for name, content := range Files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := writeIfMissing(path, []byte(content)); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// writeIfMissing writes path unless something is already there.
func writeIfMissing(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, content, 0o600)
}

// A draft, in two versions: what the branch started from, and what it looks
// like one commit later. The later one is deliberately something the
// workspace's own lint rules object to, because no-select-star is in the
// shipped .lint and a screenshot of lint working needs work to do.
const CohortsSQL = `-- Cohort report, first cut.
--
-- Still deciding whether to window by signup month or by first order,
-- so this is deliberately wide until we know.
SELECT
  *
FROM
  customers c
ORDER BY
  c.created_at DESC;
`

// The query the screenshots show, run against the demo database below.
//
// Stored as the formatter's own output. Every screenshot shows formatted SQL
// that way, without a capture having to press cmd+s and leave the file dirty
// for whichever capture runs next -- they all drive one workspace. The hero
// still presses it, and now demonstrates a formatter that agrees with itself.
//
// The window is a literal rather than `date('now', ...)`: the result set has to
// be byte-identical on every run, or each capture would produce a different
// image and every screenshot would show up as a diff.
const WeeklyRevenueSQL = `-- revenue by week, this quarter
SELECT
  date(o.created_at, 'weekday 0', '-6 days') AS week,
  printf('$%.2f', SUM(o.total_cents) / 100.0) AS revenue,
  COUNT(DISTINCT o.customer_id) AS buyers,
  COUNT(*) AS orders
FROM
  orders o
WHERE
  o.created_at >= '2026-01-05'
GROUP BY
  week
ORDER BY
  week DESC;
`

const TopCustomersSQL = `SELECT
  c.email,
  COUNT(*) AS orders,
  printf('$%.2f', SUM(o.total_cents) / 100.0) AS spend
FROM
  orders o
  JOIN customers c ON c.id = o.customer_id
GROUP BY
  c.email
ORDER BY
  spend DESC
LIMIT
  20;
`

// writeDatabase builds the SQLite database the queries read, and wires it into
// the workspace as a database instance. Without it the workspace has files to
// open and nothing to run them against.
//
// The database file lives in the app's data directory rather than in the
// workspace, so it never shows up in the file tree and a first `git add` never
// sweeps up a binary. The workspace holds only the db.config.json pointing at
// it, and the .env resolving the path.
func writeDatabase(root string) error {
	dataDir, err := utils.GetAppDataDir()
	if err != nil {
		return err
	}
	dbPath := filepath.Join(dataDir, "warehouse.db")
	// Only ever created, never rebuilt. A second call finds the rows somebody
	// has been working with, and building the schema over them fails outright
	// rather than quietly replacing them.
	if _, err := os.Stat(dbPath); err != nil {
		if err := writeDemoData(dbPath); err != nil {
			return fmt.Errorf("sample database: %w", err)
		}
	}

	// A directory containing a db.config.json is how the workspace graph
	// recognises a database instance (see graph.CheckIsDBInstance).
	dbDir := filepath.Join(root, WarehouseName)
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return err
	}
	// Written as the literal bytes the app itself writes, not marshalled from
	// graph.FSDBConfig.
	//
	// The struct's zero value produces a shorter file: `proxified` is
	// `omitempty` so false disappears, and `ssh` is a nil pointer so the block
	// disappears with it. The app fills both in the moment the connection form
	// opens, which made the workspace dirty in every capture that ran after the
	// one photographing that form -- an extra modified file in the source
	// control panel, in a screenshot about something else.
	//
	// The DSN is a $VAR resolved from the workspace .env at connection time: the
	// pattern the docs recommend, and the one the screenshots should show, since
	// it is what keeps credentials out of a committed file.
	config := `{
  "id": "` + WarehouseID + `",
  "name": "warehouse",
  "db_type": "sqlite",
  "dsn": "$WAREHOUSE_DSN",
  "ssh": {
    "enabled": false,
    "host": "",
    "port": 22,
    "user": "",
    "auth_method": "agent",
    "password": "",
    "private_key": "",
    "key_path": "",
    "host_key": ""
  },
  "proxified": false
}`
	if err := writeIfMissing(filepath.Join(dbDir, "db.config.json"), []byte(config)); err != nil {
		return err
	}

	// Secrets live in the workspace .env, never in db.config.json. The sample
	// writes only the DSN; anything else that belongs in here -- an AI provider
	// key, the e2e fixture's placeholder -- is appended by whoever owns it.
	env := "# Connection secrets. Referenced as $VAR from db.config.json.\n" +
		"WAREHOUSE_DSN=file:" + dbPath + "\n"
	if err := writeIfMissing(filepath.Join(root, ".env"), []byte(env)); err != nil {
		return err
	}

	// The sidecar is what associates a .sql file with a database, the same file
	// the app writes when you pick one from the database picker.
	meta, err := json.MarshalIndent(graph.FileMetadata{
		Databases: []graph.DatabaseRef{{Name: WarehouseName, ID: WarehouseID}},
	}, "", "  ")
	if err != nil {
		return err
	}
	for name := range Files {
		if err := writeIfMissing(filepath.Join(root, name+".metadata.json"), meta); err != nil {
			return err
		}
	}
	return nil
}

// writeDemoData creates the demo schema and fills it with fixed data.
//
// Everything here is deterministic — no clock, no randomness — because the
// screenshots are committed: a run that produced different numbers would show
// up as a changed image in every diff.

// writeDemoData creates the demo schema and fills it with fixed data.
//
// Everything here is deterministic — no clock, no randomness — because the
// screenshots are committed: a run that produced different numbers would show
// up as a changed image in every diff.
func writeDemoData(path string) error {
	handle, err := openSampleDB(path)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()

	schema := `
CREATE TABLE customers (
    id           INTEGER PRIMARY KEY,
    email        TEXT    NOT NULL UNIQUE,
    country_code TEXT    NOT NULL,
    created_at   TEXT    NOT NULL
);
CREATE TABLE orders (
    id          INTEGER PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id),
    status      TEXT    NOT NULL,
    total_cents INTEGER NOT NULL,
    created_at  TEXT    NOT NULL
);
CREATE TABLE order_items (
    id         INTEGER PRIMARY KEY,
    order_id   INTEGER NOT NULL REFERENCES orders(id),
    sku        TEXT    NOT NULL,
    quantity   INTEGER NOT NULL,
    unit_cents INTEGER NOT NULL
);
CREATE INDEX orders_customer_idx ON orders(customer_id);
`
	if _, err := handle.Exec(schema); err != nil {
		return fmt.Errorf("schema: %w", err)
	}

	tx, err := handle.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// A small deterministic sequence, so the numbers look like trade rather than
	// like a fixture, and never move.
	next := lcg(20260105)

	countries := []string{"US", "GB", "DE", "FR", "CA", "NL", "AU", "SE"}
	const customerCount = 240
	for i := 1; i <= customerCount; i++ {
		day := time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(next()%180))
		if _, err := tx.Exec(
			`INSERT INTO customers (id, email, country_code, created_at) VALUES (?, ?, ?, ?)`,
			i,
			fmt.Sprintf("user%03d@example.com", i),
			countries[next()%uint32(len(countries))],
			day.Format("2006-01-02"),
		); err != nil {
			return fmt.Errorf("customers: %w", err)
		}
	}

	// Twelve weeks from a Monday, trending gently upward so the grid reads like
	// a real business rather than noise. Starting on a Monday matters: the query
	// buckets by week, and a mid-week start would leave a stub row on top.
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	statuses := []string{"paid", "paid", "paid", "paid", "refunded"}
	orderID, itemID := 0, 0
	for week := 0; week < 12; week++ {
		orders := 60 + week*4 + int(next()%14)
		for i := 0; i < orders; i++ {
			orderID++
			day := start.AddDate(0, 0, week*7+int(next()%7))
			total := 1800 + int(next()%9000) + week*120
			if _, err := tx.Exec(
				`INSERT INTO orders (id, customer_id, status, total_cents, created_at) VALUES (?, ?, ?, ?, ?)`,
				orderID,
				1+int(next()%customerCount),
				statuses[next()%uint32(len(statuses))],
				total,
				day.Format("2006-01-02"),
			); err != nil {
				return fmt.Errorf("orders: %w", err)
			}

			for line := 0; line < 1+int(next()%3); line++ {
				itemID++
				if _, err := tx.Exec(
					`INSERT INTO order_items (id, order_id, sku, quantity, unit_cents) VALUES (?, ?, ?, ?, ?)`,
					itemID,
					orderID,
					fmt.Sprintf("SKU-%04d", 1000+next()%400),
					1+int(next()%3),
					600+int(next()%4000),
				); err != nil {
					return fmt.Errorf("order_items: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}

// lcg returns a tiny deterministic pseudo-random source. A real PRNG would do,
// but this keeps the fixture's numbers pinned to this function rather than to
// a standard library implementation that could change between Go versions.

func lcg(seed uint32) func() uint32 {
	state := seed
	return func() uint32 {
		state = state*1664525 + 1013904223
		return state >> 8
	}
}

// writeAvatar gives the seeded user a picture, at the path
// User.GetCurrentUserAvatar reads. Without one the app logs a failed read on
// every graph load, and the account in the corner of every screenshot is a
// blank.
//
// It is drawn rather than checked in: a committed binary for six coloured
// squares is a worse trade than twenty lines that produce them, and drawing it
// keeps the fixture text-only.
