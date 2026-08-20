package schema

import (
	"strings"
	"testing"
)

func rawCol(name, typ, comment string) RawColumn {
	return RawColumn{Name: name, DataType: typ, Comment: comment}
}

func buildOne(t *testing.T, tableComment string, cols []RawColumn) (Entity, []error) {
	t.Helper()
	ents, errs := Build(RawSchema{Tables: []RawTable{{
		Schema: "app", Name: "thing", Comment: tableComment,
		PrimaryKey: []string{"id"}, Columns: cols,
	}}})
	if len(ents) != 1 {
		t.Fatalf("want 1 entity, got %d", len(ents))
	}
	return ents[0], errs
}

func TestDefaultSort(t *testing.T) {
	const list = "@app.api.list"

	t.Run("tag on a name column is the ascending default", func(t *testing.T) {
		e, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("name", "text", "@app.sort")})
		if len(errs) != 0 {
			t.Fatalf("errs: %v", errs)
		}
		if e.DefaultSort != "name" {
			t.Fatalf("DefaultSort=%q, want name", e.DefaultSort)
		}
	})

	t.Run("tag desc on a time column", func(t *testing.T) {
		e, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("occurred_at", "timestamptz", "@app.sort desc")})
		if len(errs) != 0 || e.DefaultSort != "-occurred_at" {
			t.Fatalf("DefaultSort=%q errs=%v", e.DefaultSort, errs)
		}
	})

	t.Run("no tag falls back to updated_at newest-first", func(t *testing.T) {
		e, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("updated_at", "timestamptz", "")})
		if len(errs) != 0 || e.DefaultSort != "-updated_at" {
			t.Fatalf("DefaultSort=%q errs=%v", e.DefaultSort, errs)
		}
	})

	t.Run("list endpoint with no tag and no updated_at is a lint error", func(t *testing.T) {
		e, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("label", "text", "")})
		if e.DefaultSort != "" || len(errs) == 0 {
			t.Fatalf("want a lint error, got DefaultSort=%q errs=%v", e.DefaultSort, errs)
		}
	})

	t.Run("two @app.sort columns is an error", func(t *testing.T) {
		_, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("a", "text", "@app.sort"), rawCol("b", "text", "@app.sort")})
		if !anyContains(errs, "@app.sort") {
			t.Fatalf("want a multi-sort error, got %v", errs)
		}
	})

	t.Run("@app.sort on json is an error", func(t *testing.T) {
		_, errs := buildOne(t, list, []RawColumn{rawCol("id", "uuid", ""), rawCol("payload", "jsonb", "@app.sort")})
		if !anyContains(errs, "json") {
			t.Fatalf("want a json-sort error, got %v", errs)
		}
	})

	t.Run("no list op needs no default sort", func(t *testing.T) {
		_, errs := buildOne(t, "@app.entity thing", []RawColumn{rawCol("id", "uuid", "")})
		if len(errs) != 0 {
			t.Fatalf("errs: %v", errs)
		}
	})
}

func anyContains(errs []error, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), sub) {
			return true
		}
	}
	return false
}
