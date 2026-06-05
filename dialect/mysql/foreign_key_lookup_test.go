package mysql

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
)

func TestBuildForeignKeyLookupSQL_Basic(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "shop",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Limit:          200,
	})
	want := "SELECT `id`, `email`, `name` FROM `shop`.`users` LIMIT 200"
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_WithSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "shop",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Query:          "alice",
		Limit:          50,
	})
	want := "SELECT `id`, `email`, `name` FROM `shop`.`users` WHERE (LOWER(CAST(`email` AS CHAR)) LIKE LOWER('%alice%') ESCAPE '$' OR LOWER(CAST(`name` AS CHAR)) LIKE LOWER('%alice%') ESCAPE '$') LIMIT 50"
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_PinsCurrentValue(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "shop",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email"},
		Query:          "x",
		CurrentValue:   "42",
		Limit:          200,
	})
	if !strings.Contains(sql, "OR CAST(`id` AS CHAR) = '42'") {
		t.Fatalf("missing current-value OR clause: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY CAST(`id` AS CHAR) = '42' DESC") {
		t.Fatalf("missing current-value ORDER BY: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_PinWithoutSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:       "shop",
		Table:        "users",
		FKColumn:     "id",
		CurrentValue: "42",
		Limit:        10,
	})
	want := "SELECT `id` FROM `shop`.`users` ORDER BY CAST(`id` AS CHAR) = '42' DESC LIMIT 10"
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_EmptySchemaSkipsQualification(t *testing.T) {
	// MySQL relies on the connection's currently-selected database when the
	// schema is empty; we must not prepend an empty backtick-quoted prefix.
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Table:    "users",
		FKColumn: "id",
		Limit:    1,
	})
	want := "SELECT `id` FROM `users` LIMIT 1"
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesIdentifierBackticks(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "we`ird",
		Table:          "ta`ble",
		FKColumn:       "co`l",
		DisplayColumns: []string{"d`isp"},
		Limit:          1,
	})
	want := "SELECT `co``l`, `d``isp` FROM `we``ird`.`ta``ble` LIMIT 1"
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_DoublesBackslashInLiteral(t *testing.T) {
	d := NewDialect()
	// In MySQL with default sql_mode, '\X' inside a literal is processed as
	// an escape *before* LIKE sees it. To make user input arrive at the LIKE
	// matcher byte-for-byte, every backslash in the literal must be doubled.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "id",
		Query:    `pa\th`,
		Limit:    1,
	})
	if !strings.Contains(sql, `'%pa\\th%'`) {
		t.Fatalf("expected backslash to be doubled in literal: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_DoublesBackslashInCurrentValue(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:       "shop",
		Table:        "users",
		FKColumn:     "id",
		CurrentValue: `back\slash`,
		Limit:        1,
	})
	if !strings.Contains(sql, "CAST(`id` AS CHAR) = 'back\\\\slash'") {
		t.Fatalf("expected backslash to be doubled in CurrentValue literal: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesLikeWildcards(t *testing.T) {
	d := NewDialect()
	// % and _ in user input must match literally, even after MySQL's literal
	// parser has consumed them, because we picked a non-backslash escape
	// character ($) that survives literal parsing untouched.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "name",
		Query:    "100%_off",
		Limit:    1,
	})
	if !strings.Contains(sql, `'%100$%$_off%'`) {
		t.Fatalf("expected wildcards to be escaped with $, got: %s", sql)
	}
	if !strings.Contains(sql, `ESCAPE '$'`) {
		t.Fatalf("missing ESCAPE clause: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesEscapeCharInQuery(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "id",
		Query:    "$money",
		Limit:    1,
	})
	if !strings.Contains(sql, `'%$$money%'`) {
		t.Fatalf("expected $ to be doubled, got: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesLiteralQuotes(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "id",
		Query:    "O'Brien",
		Limit:    1,
	})
	if !strings.Contains(sql, `'%O''Brien%'`) {
		t.Fatalf("expected doubled-quote escape, got: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoLimit(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "id",
	})
	if strings.Contains(sql, "LIMIT") {
		t.Fatalf("did not expect LIMIT: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_FKColumnDedupedInSelect(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "shop",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"id", "email"},
		Limit:          10,
	})
	if !strings.HasPrefix(sql, "SELECT `id`, `email` FROM") {
		t.Fatalf("expected deduped SELECT list, got: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoTrailingSemicolon(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "shop",
		Table:    "users",
		FKColumn: "id",
		Limit:    1,
	})
	if strings.HasSuffix(sql, ";") {
		t.Fatalf("statement must not end with ;: %q", sql)
	}
}
