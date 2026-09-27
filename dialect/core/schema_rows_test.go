package core

import (
	"encoding/json"
	"testing"
)

func TestSchemaRowsMetadata(t *testing.T) {
	var r SchemaRows
	for _, row := range []struct{ kind, doc string }{
		{"schema", `{"name":"pg_catalog","catalog":true}`},
		{"schema", `{"name":"b"}`},
		{"schema", `{"name":"A"}`},
		{"current", `{"name":"b"}`},
		{"relation", `{"schema":"b","name":"t","kind":"table","ddl":"CREATE TABLE t"}`},
		{"relation", `{"schema":"b","name":"v","kind":"view"}`},
		{"relation", `{"schema":"gone","name":"x","kind":"table"}`},
		{"column", `{"schema":"b","table":"t","name":"y","type":"int","nullable":1,"position":2}`},
		{"column", `{"schema":"b","table":"t","name":"x","type":"int","nullable":0,"position":1}`},
		{"column", `{"schema":"b","table":"v","name":"x","type":"int","nullable":true,"position":1}`},
		{"primary_key", `{"schema":"b","table":"t","column":"x","position":1}`},
		{"foreign_key", `{"schema":"b","table":"t","column":"y","refSchema":"A","refTable":"p","refColumn":"id"}`},
		{"index", `{"schema":"b","table":"t","name":"i"}`},
		{"index_column", `{"schema":"b","table":"t","index":"i","name":"","position":1}`},
		{"index_column", `{"schema":"b","table":"t","index":"i","name":"y","position":2,"descending":true}`},
		{"stat", `{"schema":"b","name":"t","value":"7"}`},
		{"setting", `{"name":"work_mem","value":"4MB"}`},
		{"extra_kind", `{"any":1}`},
	} {
		if err := decodeRow(&r, row.kind, []byte(row.doc)); err != nil {
			t.Fatalf("%s: %v", row.kind, err)
		}
	}
	meta := r.Metadata()

	var names []string
	for _, s := range meta.Schemas {
		names = append(names, s.Name)
	}
	if got := names; len(got) != 3 || got[0] != "A" || got[1] != "b" || got[2] != "pg_catalog" {
		t.Fatalf("schemas = %v, want user schemas by name then the catalog", got)
	}
	b, catalog := meta.Schemas[1], meta.Schemas[2]
	if meta.CurrentSchema != "b" || len(catalog.Settings) != 1 || b.Settings != nil {
		t.Errorf("current = %q, catalog settings = %v, b settings = %v", meta.CurrentSchema, catalog.Settings, b.Settings)
	}
	if len(b.Tables) != 1 || len(b.Views) != 1 {
		t.Fatalf("tables = %+v, views = %+v", b.Tables, b.Views)
	}
	tbl := b.Tables[0]
	if tbl.Columns[0].Name != "x" || !tbl.Columns[0].IsPrimaryKey || tbl.Columns[0].Nullable || !tbl.Columns[1].Nullable {
		t.Errorf("columns = %+v", tbl.Columns)
	}
	if fk := tbl.Columns[1].ForeignKey; fk == nil || fk.SchemaName != "A" || fk.TableName != "p" {
		t.Errorf("foreign key = %+v", fk)
	}
	if b.Views[0].Columns[0].IsPrimaryKey || b.Views[0].PrimaryKey != nil {
		t.Errorf("a view has no keys: %+v", b.Views[0])
	}
	if idx := b.Indexes; len(idx) != 1 || len(idx[0].Columns) != 1 || !idx[0].Columns[0].Descending {
		t.Errorf("indexes = %+v, want the expression column left out", idx)
	}
	if b.Stats["t"] != "7" || b.ForeignTables == nil || catalog.ForeignTables != nil {
		t.Errorf("stats = %v, foreign tables = %v / %v", b.Stats, b.ForeignTables, catalog.ForeignTables)
	}
	if len(r.Other["extra_kind"]) != 1 {
		t.Errorf("unknown kinds are kept for the dialect: %v", r.Other)
	}
}

func TestFlagRejectsNonBooleans(t *testing.T) {
	var f Flag
	if err := json.Unmarshal([]byte(`"yes"`), &f); err == nil {
		t.Error(`"yes" read as a boolean`)
	}
}
