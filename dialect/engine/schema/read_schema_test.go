package schema

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

// A schema load is one query that must stay right and flat on every server
// version, so each dialect reads the same fixture here. SQLite always runs;
// Postgres and MySQL run when CI points SCHEMA_TEST_POSTGRES_DSN or
// SCHEMA_TEST_MYSQL_DSN at a server whose databases the test may drop.

type schemaTarget struct {
	name    string
	dialect core.SQLDialect
	dsn     string
	// setupDSN runs the fixture, several statements per Exec.
	setupDSN string
	fixture  string
	// dropFixture undoes the fixture, so a rerun starts clean.
	dropFixture string
	// scaleTables returns the DDL of n tables in the scale schema.
	scaleTables func(n int) string

	schemaA, schemaB, schemaEmpty, scaleSchema, catalog, current string
	maxQueries                                                   int

	crossSchema, comments, enumType, expressionIndex, functions, stats, settings bool
}

func schemaTargets(t *testing.T) []schemaTarget {
	targets := []schemaTarget{sqliteTarget(t)}
	if dsn := os.Getenv("SCHEMA_TEST_POSTGRES_DSN"); dsn != "" {
		targets = append(targets, postgresTarget(dsn))
	}
	if dsn := os.Getenv("SCHEMA_TEST_MYSQL_DSN"); dsn != "" {
		targets = append(targets, mysqlTarget(dsn))
	}
	return targets
}

func sqliteTarget(t *testing.T) schemaTarget {
	dsn := filepath.Join(t.TempDir(), "fixture.db")
	return schemaTarget{
		name: "sqlite", dialect: sqlite.NewDialect(), dsn: dsn, setupDSN: dsn,
		fixture: `
			CREATE TABLE parent (id INTEGER PRIMARY KEY, code TEXT NOT NULL DEFAULT 'x' UNIQUE, "Mixed Case" INT);
			CREATE TABLE child (a INT NOT NULL, b INT NOT NULL, parent_id INT REFERENCES parent(id), note TEXT, PRIMARY KEY (b, a));
			CREATE INDEX idx_child ON child (parent_id, a DESC);
			CREATE INDEX idx_expr ON parent (lower(code));
			CREATE VIEW v_child AS SELECT a, b FROM child WHERE a > 1;
			CREATE TRIGGER trg_child AFTER INSERT ON child BEGIN SELECT 1; END;`,
		dropFixture: `
			DROP VIEW IF EXISTS v_child; DROP TABLE IF EXISTS child; DROP TABLE IF EXISTS parent;
			DROP TABLE IF EXISTS scale_parent;` + dropScaleTables(func(i int) string { return fmt.Sprintf("DROP TABLE IF EXISTS scale_t%d;", i) }),
		scaleTables: func(n int) string {
			var b strings.Builder
			b.WriteString("CREATE TABLE IF NOT EXISTS scale_parent (id INTEGER PRIMARY KEY);")
			for i := range n {
				fmt.Fprintf(&b, `CREATE TABLE scale_t%d (id INTEGER PRIMARY KEY, a INT NOT NULL DEFAULT 0, b TEXT,
					parent INT REFERENCES scale_parent(id)); CREATE INDEX scale_t%d_ab ON scale_t%d (a, b);`, i, i, i)
			}
			return b.String()
		},
		schemaA: "main", schemaB: "", schemaEmpty: "", scaleSchema: "main", catalog: "sqlite_builtin", current: "main",
		maxQueries: 1,
		// SQLite has one schema, no comments, no named types and no stored functions.
		expressionIndex: true, settings: true,
	}
}

func postgresTarget(dsn string) schemaTarget {
	return schemaTarget{
		name: "postgres", dialect: postgresql.NewDialect(), dsn: dsn, setupDSN: dsn,
		fixture: `
			CREATE SCHEMA selt_a; CREATE SCHEMA selt_b; CREATE SCHEMA selt_empty;
			CREATE TYPE selt_a.mood AS ENUM ('sad', 'ok', 'it''s');
			CREATE TABLE selt_a.parent (id int PRIMARY KEY, code text NOT NULL DEFAULT 'x' UNIQUE, mood selt_a.mood, "Mixed Case" int);
			COMMENT ON TABLE selt_a.parent IS 'parent table';
			CREATE TABLE selt_a.child (a int NOT NULL, b int NOT NULL, parent_id int REFERENCES selt_a.parent(id), note text, PRIMARY KEY (b, a));
			COMMENT ON COLUMN selt_a.child.note IS 'a note';
			CREATE INDEX idx_child ON selt_a.child (parent_id, a DESC);
			CREATE INDEX idx_expr ON selt_a.parent (lower(code));
			CREATE VIEW selt_a.v_child AS SELECT a, b FROM selt_a.child WHERE a > 1;
			CREATE FUNCTION selt_a.add_xy(x int, y int) RETURNS int LANGUAGE sql AS 'SELECT x + y';
			CREATE FUNCTION selt_a.tg() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RETURN NEW; END$$;
			CREATE TRIGGER trg_child BEFORE INSERT ON selt_a.child FOR EACH ROW EXECUTE FUNCTION selt_a.tg();
			CREATE TABLE selt_b.parent (id bigint PRIMARY KEY, ref int REFERENCES selt_a.parent(id));`,
		dropFixture: `DROP SCHEMA IF EXISTS selt_a, selt_b, selt_empty, selt_scale CASCADE;`,
		scaleTables: func(n int) string {
			var b strings.Builder
			b.WriteString("CREATE SCHEMA selt_scale; CREATE TABLE selt_scale.parent (id int PRIMARY KEY);")
			for i := range n {
				fmt.Fprintf(&b, `CREATE TABLE selt_scale.t%d (id int PRIMARY KEY, a int NOT NULL DEFAULT 0, b text,
					parent int REFERENCES selt_scale.parent(id)); CREATE INDEX ON selt_scale.t%d (a, b);`, i, i)
			}
			return b.String()
		},
		schemaA: "selt_a", schemaB: "selt_b", schemaEmpty: "selt_empty", scaleSchema: "selt_scale",
		catalog: "pg_catalog", current: "public", maxQueries: 1,
		crossSchema: true, comments: true, enumType: true, expressionIndex: true, functions: true, stats: true, settings: true,
	}
}

func mysqlTarget(dsn string) schemaTarget {
	return schemaTarget{
		name: "mysql", dialect: mysql.NewDialect(), dsn: dsn, setupDSN: withMySQLParam(dsn, "multiStatements=true"),
		fixture: `
			CREATE DATABASE selt_a; CREATE DATABASE selt_b; CREATE DATABASE selt_empty;
			CREATE TABLE selt_a.parent (id INT NOT NULL, code VARCHAR(20) NOT NULL DEFAULT 'x', mood ENUM('sad','ok','it''s'),
				` + "`Mixed Case`" + ` INT, PRIMARY KEY (id), UNIQUE KEY uq_code (code)) COMMENT='parent table';
			CREATE TABLE selt_a.child (a INT NOT NULL, b INT NOT NULL, parent_id INT, note TEXT COMMENT 'a note',
				PRIMARY KEY (b, a), KEY idx_child (parent_id, a DESC),
				CONSTRAINT fk_parent FOREIGN KEY (parent_id) REFERENCES selt_a.parent(id) ON DELETE CASCADE);
			CREATE VIEW selt_a.v_child AS SELECT a, b FROM selt_a.child WHERE a > 1;
			CREATE TRIGGER selt_a.trg_child BEFORE INSERT ON selt_a.child FOR EACH ROW SET NEW.a = NEW.a + 1;
			CREATE FUNCTION selt_a.add_xy(x INT, y INT) RETURNS INT DETERMINISTIC RETURN x + y;
			CREATE TABLE selt_b.parent (id BIGINT PRIMARY KEY, ref INT, CONSTRAINT fk_x FOREIGN KEY (ref) REFERENCES selt_a.parent(id));`,
		dropFixture: `DROP DATABASE IF EXISTS selt_b; DROP DATABASE IF EXISTS selt_a; DROP DATABASE IF EXISTS selt_empty;
			DROP DATABASE IF EXISTS selt_scale; DROP DATABASE IF EXISTS selt_rt;`,
		scaleTables: func(n int) string {
			var b strings.Builder
			b.WriteString("CREATE DATABASE selt_scale; CREATE TABLE selt_scale.parent (id INT PRIMARY KEY);")
			for i := range n {
				fmt.Fprintf(&b, `CREATE TABLE selt_scale.t%d (id INT PRIMARY KEY, a INT NOT NULL DEFAULT 0, b VARCHAR(20),
					parent INT, KEY k_ab (a, b), CONSTRAINT fk_t%d FOREIGN KEY (parent) REFERENCES selt_scale.parent(id));`, i, i)
			}
			return b.String()
		},
		schemaA: "selt_a", schemaB: "selt_b", schemaEmpty: "selt_empty", scaleSchema: "selt_scale",
		catalog: "mysql_builtin", current: "", maxQueries: 3,
		// MySQL comments live in the DDL; the settings may be unreadable.
		crossSchema: true, functions: true, stats: true,
	}
}

func dropScaleTables(drop func(int) string) string {
	var b strings.Builder
	for i := range scaleLarge {
		b.WriteString(drop(i))
	}
	return b.String()
}

func withMySQLParam(dsn, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}
	return dsn + "?" + param
}

// setUp runs the fixture on a clean server and returns a pool that counts the
// queries it sends.
func (tg schemaTarget) setUp(t *testing.T, ddl string) (*sql.DB, *atomic.Int64) {
	t.Helper()
	setup := tg.open(t, tg.setupDSN)
	if _, err := setup.Exec(tg.dropFixture); err != nil {
		t.Fatalf("drop fixture: %v", err)
	}
	t.Cleanup(func() { _, _ = setup.Exec(tg.dropFixture) })
	if _, err := setup.Exec(ddl); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	plain := tg.open(t, tg.dsn)
	queries := new(atomic.Int64)
	db := sql.OpenDB(countingConnector{driver: plain.Driver(), dsn: tg.dsn, queries: queries})
	t.Cleanup(func() { _ = db.Close() })
	return db, queries
}

// open waits for a server that CI may still be starting.
func (tg schemaTarget) open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := tg.dialect.OpenDB(dsn)
	for deadline := time.Now().Add(90 * time.Second); err == nil; {
		if err = db.Ping(); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("%s: %v", tg.name, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestReadSchemaFixture(t *testing.T) {
	for _, tg := range schemaTargets(t) {
		t.Run(tg.name, func(t *testing.T) {
			db, queries := tg.setUp(t, tg.fixture)
			meta, err := Fetch(context.Background(), db, tg.dialect, "db")
			if err != nil {
				t.Fatal(err)
			}
			if n := queries.Load(); n > int64(tg.maxQueries) {
				t.Errorf("a load sent %d queries, want at most %d", n, tg.maxQueries)
			}
			tg.checkFixture(t, meta)
		})
	}
}

func (tg schemaTarget) checkFixture(t *testing.T, meta *core.Metadata) {
	if meta.CurrentSchema != tg.current || meta.DefaultSchema != tg.current {
		t.Errorf("current = %q, default = %q, want %q", meta.CurrentSchema, meta.DefaultSchema, tg.current)
	}
	last := meta.Schemas[len(meta.Schemas)-1]
	if last.Name != tg.catalog || len(last.Types) == 0 || len(last.Functions) == 0 {
		t.Errorf("last schema = %q with %d types, %d functions, want the %s catalog", last.Name, len(last.Types), len(last.Functions), tg.catalog)
	}
	if tg.settings && len(last.Settings) == 0 {
		t.Error("the catalog has no settings")
	}

	a := findSchema(t, meta, tg.schemaA)
	if got := tableNames(a.Tables); !slices.Equal(got, []string{"child", "parent"}) {
		t.Fatalf("%s tables = %v", tg.schemaA, got)
	}
	parent, child := a.Tables[1], a.Tables[0]

	wantParent := []string{"id", "code", "Mixed Case"}
	if tg.enumType || tg.name == "mysql" {
		wantParent = []string{"id", "code", "mood", "Mixed Case"}
	}
	if got := columnNames(parent.Columns); !slices.Equal(got, wantParent) {
		t.Errorf("parent columns = %v, want %v", got, wantParent)
	}
	if !slices.Equal(parent.PrimaryKey, []string{"id"}) || !parent.Columns[0].IsPrimaryKey {
		t.Errorf("parent key = %v", parent.PrimaryKey)
	}
	if code := parent.Columns[1]; code.Nullable || code.Default == nil {
		t.Errorf("code = %+v, want NOT NULL with a default", code)
	}
	if len(wantParent) == 4 && !slices.Equal(parent.Columns[2].EnumValues, []string{"sad", "ok", "it's"}) {
		t.Errorf("mood enum values = %q", parent.Columns[2].EnumValues)
	}

	if got := columnNames(child.Columns); !slices.Equal(got, []string{"a", "b", "parent_id", "note"}) {
		t.Errorf("child columns = %v", got)
	}
	if !slices.Equal(child.PrimaryKey, []string{"b", "a"}) {
		t.Errorf("child key = %v, want [b a] in key order", child.PrimaryKey)
	}
	fk := child.Columns[2].ForeignKey
	if fk == nil || fk.SchemaName != tg.schemaA || fk.TableName != "parent" || fk.ColumnName != "id" {
		t.Errorf("parent_id foreign key = %+v", fk)
	}
	if !strings.Contains(parent.DDL, "CREATE TABLE") || !strings.Contains(child.DDL, "parent_id") {
		t.Errorf("table DDL:\n%s\n%s", parent.DDL, child.DDL)
	}
	if tg.comments && (parent.Description != "parent table" || child.Columns[3].Description != "a note") {
		t.Errorf("comments = %q, %q", parent.Description, child.Columns[3].Description)
	}

	if len(a.Views) != 1 || !slices.Equal(columnNames(a.Views[0].Columns), []string{"a", "b"}) ||
		!strings.Contains(strings.ToUpper(a.Views[0].DDL), "VIEW") {
		t.Errorf("views = %+v", a.Views)
	}

	indexes := map[string]core.IndexInfo{}
	for _, idx := range a.Indexes {
		indexes[idx.Name] = idx
	}
	idx := indexes["idx_child"]
	if idx.TableName != "child" || len(idx.Columns) != 2 || idx.Columns[0].Name != "parent_id" ||
		idx.Columns[1].Name != "a" || !idx.Columns[1].Descending || idx.Columns[0].Descending {
		t.Errorf("idx_child = %+v", idx)
	}
	if tg.expressionIndex {
		if expr, ok := indexes["idx_expr"]; !ok || len(expr.Columns) != 0 {
			t.Errorf("idx_expr = %+v, want the index without its expression column", expr)
		}
	}
	unique := false
	for _, idx := range a.Indexes {
		unique = unique || (idx.TableName == "parent" && len(idx.Columns) == 1 && idx.Columns[0].Name == "code")
	}
	if !unique {
		t.Errorf("no index on parent(code): %+v", a.Indexes)
	}

	if len(a.Triggers) != 1 || a.Triggers[0].Name != "trg_child" || a.Triggers[0].TableName != "child" ||
		!strings.Contains(a.Triggers[0].DDL, "trg_child") {
		t.Errorf("triggers = %+v", a.Triggers)
	}
	if tg.stats {
		if _, ok := a.Stats["child"]; !ok {
			t.Errorf("stats = %v, want an entry for child", a.Stats)
		}
	}
	if tg.enumType {
		found := false
		for _, typ := range a.Types {
			found = found || (typ.Name == "mood" && slices.Equal(typ.EnumLabels, []string{"sad", "ok", "it's"}))
		}
		if !found {
			t.Errorf("types = %+v, want mood with its labels", a.Types)
		}
	}
	if tg.functions {
		var add *core.Function
		for i := range a.Functions {
			if a.Functions[i].Name == "add_xy" {
				add = &a.Functions[i]
			}
		}
		if add == nil || !strings.Contains(add.Args, "x ") || !strings.Contains(add.Args, "y ") {
			t.Errorf("functions = %+v, want add_xy(x, y)", a.Functions)
		}
	}

	if tg.crossSchema {
		b := findSchema(t, meta, tg.schemaB)
		if len(b.Tables) != 1 || !slices.Equal(columnNames(b.Tables[0].Columns), []string{"id", "ref"}) {
			t.Fatalf("%s tables = %+v, want its own parent", tg.schemaB, b.Tables)
		}
		ref := b.Tables[0].Columns[1].ForeignKey
		if ref == nil || ref.SchemaName != tg.schemaA || ref.TableName != "parent" || ref.ColumnName != "id" {
			t.Errorf("cross-schema foreign key = %+v", ref)
		}
		if empty := findSchema(t, meta, tg.schemaEmpty); len(empty.Tables)+len(empty.Views) != 0 {
			t.Errorf("%s is not empty: %+v", tg.schemaEmpty, empty)
		}
	}
}

const scaleSmall, scaleLarge = 100, 400

// A load's cost must grow with the schema, not with its square. Allocations
// are steadier than time on a shared runner, and the cost of each extra table
// leaves out the catalog every load pays: it stays flat from 100 to 400 tables
// when growth is linear, and rises several times over when it is quadratic.
func TestReadSchemaScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("creates hundreds of tables")
	}
	for _, tg := range schemaTargets(t) {
		t.Run(tg.name, func(t *testing.T) {
			sizes := []int{0, scaleSmall, scaleLarge}
			allocs := make([]float64, len(sizes))
			for i, n := range sizes {
				db, queries := tg.setUp(t, tg.scaleTables(n))
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				meta, err := Fetch(context.Background(), db, tg.dialect, "db")
				runtime.ReadMemStats(&after)
				if err != nil {
					t.Fatal(err)
				}
				allocs[i] = float64(after.TotalAlloc - before.TotalAlloc)
				if got := len(findSchema(t, meta, tg.scaleSchema).Tables); got < n {
					t.Fatalf("%d tables read, want at least %d", got, n)
				}
				if q := queries.Load(); q > int64(tg.maxQueries) {
					t.Errorf("%d tables took %d queries, want at most %d", n, q, tg.maxQueries)
				}
			}
			small := (allocs[1] - allocs[0]) / scaleSmall
			large := (allocs[2] - allocs[1]) / (scaleLarge - scaleSmall)
			if large > 3*small {
				t.Errorf("each table cost %.0f bytes up to %d tables and %.0f bytes up to %d", small, scaleSmall, large, scaleLarge)
			}
		})
	}
}

func TestReadSchemaCancelReleasesTheConnection(t *testing.T) {
	for _, tg := range schemaTargets(t) {
		t.Run(tg.name, func(t *testing.T) {
			db, _ := tg.setUp(t, tg.fixture)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := Fetch(ctx, db, tg.dialect, "db"); err == nil {
				t.Error("a cancelled load succeeded")
			}
			if inUse := db.Stats().InUse; inUse != 0 {
				t.Errorf("%d connections still in use", inUse)
			}
			if _, err := Fetch(context.Background(), db, tg.dialect, "db"); err != nil {
				t.Errorf("the load after a cancel failed: %v", err)
			}
		})
	}
}

// The MySQL DDL is rebuilt from information_schema, so running it must give
// back the tables it was read from.
func TestReadSchemaMySQLDDLRecreatesTheTables(t *testing.T) {
	var tg schemaTarget
	for _, candidate := range schemaTargets(t) {
		if candidate.name == "mysql" {
			tg = candidate
		}
	}
	if tg.name == "" {
		t.Skip("SCHEMA_TEST_MYSQL_DSN is not set")
	}
	db, _ := tg.setUp(t, tg.fixture)
	meta, err := Fetch(context.Background(), db, tg.dialect, "db")
	if err != nil {
		t.Fatal(err)
	}
	original := findSchema(t, meta, tg.schemaA)

	ctx := context.Background()
	conn, err := tg.open(t, tg.setupDSN).Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	statements := []string{"SET FOREIGN_KEY_CHECKS = 0", "CREATE DATABASE selt_rt", "USE selt_rt"}
	for _, table := range original.Tables {
		statements = append(statements, table.DDL)
	}
	for _, view := range original.Views {
		statements = append(statements, view.DDL)
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("%v running:\n%s", err, statement)
		}
	}

	meta, err = Fetch(ctx, db, tg.dialect, "db")
	if err != nil {
		t.Fatal(err)
	}
	recreated := findSchema(t, meta, "selt_rt")
	if len(recreated.Tables) != len(original.Tables) || len(recreated.Views) != len(original.Views) {
		t.Fatalf("recreated %d tables and %d views from %d and %d", len(recreated.Tables), len(recreated.Views), len(original.Tables), len(original.Views))
	}
	for i, want := range original.Tables {
		got := recreated.Tables[i]
		if strings.ReplaceAll(got.DDL, "selt_rt", tg.schemaA) != want.DDL {
			t.Errorf("%s DDL differs once recreated:\n%s\nfrom:\n%s", want.Name, got.DDL, want.DDL)
		}
		for j, c := range want.Columns {
			g := got.Columns[j]
			if g.Name != c.Name || g.Type != c.Type || g.Nullable != c.Nullable || !equalDefault(g.Default, c.Default) || g.IsPrimaryKey != c.IsPrimaryKey {
				t.Errorf("%s.%s = %+v, want %+v", want.Name, c.Name, g, c)
			}
		}
	}
	for i, want := range original.Views {
		if got := recreated.Views[i]; !slices.Equal(columnNames(got.Columns), columnNames(want.Columns)) {
			t.Errorf("view %s columns = %v, want %v", want.Name, columnNames(got.Columns), columnNames(want.Columns))
		}
	}
}

func equalDefault(a, b *string) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

func findSchema(t *testing.T, meta *core.Metadata, name string) core.Schema {
	t.Helper()
	for _, s := range meta.Schemas {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no schema %q", name)
	return core.Schema{}
}

func tableNames(tables []core.Table) []string {
	var names []string
	for _, tbl := range tables {
		names = append(names, tbl.Name)
	}
	return names
}

func columnNames(columns []core.Column) []string {
	var names []string
	for _, c := range columns {
		names = append(names, c.Name)
	}
	return names
}

// countingConnector counts what a pool sends as a query or a prepare, the
// round trips a schema load is meant to keep flat.
type countingConnector struct {
	driver  driver.Driver
	dsn     string
	queries *atomic.Int64
}

func (c countingConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return countingConn{Conn: conn, queries: c.queries}, nil
}

func (c countingConnector) Driver() driver.Driver { return c.driver }

type countingConn struct {
	driver.Conn
	queries *atomic.Int64
}

func (c countingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.queries.Add(1)
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	c.queries.Add(1)
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
}
