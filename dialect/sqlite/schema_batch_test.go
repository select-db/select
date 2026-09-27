package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// The one-query-per-kind reads must keep what per-relation reads gave: column
// order, key order, foreign keys (an implicit target included) and index columns.
func TestSchemaReadsKeepOrderAndKeys(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`
		CREATE TABLE parent (a TEXT, b TEXT, PRIMARY KEY (b, a));
		CREATE TABLE child (id INTEGER PRIMARY KEY, pa TEXT REFERENCES parent, qa TEXT REFERENCES parent(a));
		CREATE INDEX child_idx ON child (qa DESC, id);
		CREATE VIEW v AS SELECT id, qa FROM child;
	`); err != nil {
		t.Fatal(err)
	}
	d, ctx := NewDialect(), context.Background()

	tables, err := d.GetTables(ctx, db, "main")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]core.Table{}
	for _, tbl := range tables {
		byName[tbl.Name] = tbl
	}
	parent, child := byName["parent"], byName["child"]
	if len(tables) != 2 || len(parent.Columns) != 2 || len(child.Columns) != 3 {
		t.Fatalf("tables = %+v", tables)
	}
	if parent.Columns[0].Name != "a" || parent.Columns[1].Name != "b" {
		t.Errorf("columns out of table order: %+v", parent.Columns)
	}
	if len(parent.PrimaryKey) != 2 || parent.PrimaryKey[0] != "b" || parent.PrimaryKey[1] != "a" {
		t.Errorf("primary key = %v, want [b a]", parent.PrimaryKey)
	}
	fk := map[string]string{}
	for _, c := range child.Columns {
		if c.IsForeignKey {
			fk[c.Name] = c.ForeignKey.TableName + "." + c.ForeignKey.ColumnName
		}
	}
	if fk["pa"] != "parent." || fk["qa"] != "parent.a" {
		t.Errorf("foreign keys = %v", fk)
	}

	views, err := d.GetViews(ctx, db, "main")
	if err != nil || len(views) != 1 || len(views[0].Columns) != 2 {
		t.Fatalf("views = %+v, %v", views, err)
	}

	indexes, err := d.GetIndexes(ctx, db, "main")
	if err != nil {
		t.Fatal(err)
	}
	var idx []string
	for _, i := range indexes {
		if i.Name == "child_idx" {
			for _, c := range i.Columns {
				if c.Descending {
					idx = append(idx, c.Name+" desc")
				} else {
					idx = append(idx, c.Name)
				}
			}
		}
	}
	if len(idx) < 2 || idx[0] != "qa desc" || idx[1] != "id" {
		t.Errorf("child_idx columns = %v, want [qa desc, id, ...]", idx)
	}
}
