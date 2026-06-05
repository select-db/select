package postgresql

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
)

func TestBuildForeignKeyLookupSQL_Basic(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Limit:          200,
	})
	want := `SELECT "id", "email", "name" FROM "public"."users" LIMIT 200`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_WithSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Query:          "alice",
		Limit:          50,
	})
	want := `SELECT "id", "email", "name" FROM "public"."users" WHERE ("email"::text ILIKE '%alice%' ESCAPE '$' OR "name"::text ILIKE '%alice%' ESCAPE '$') LIMIT 50`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_PinsCurrentValue(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email"},
		Query:          "x",
		CurrentValue:   "42",
		Limit:          200,
	})
	// Pinning should appear (a) as an OR-clause in WHERE so the row survives
	// the search filter, and (b) as the leading ORDER BY so it floats to top.
	if !strings.Contains(sql, `OR "id"::text = '42'`) {
		t.Fatalf("missing current-value OR clause: %s", sql)
	}
	if !strings.Contains(sql, `ORDER BY "id"::text = '42' DESC`) {
		t.Fatalf("missing current-value ORDER BY: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_PinWithoutSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:       "public",
		Table:        "users",
		FKColumn:     "id",
		CurrentValue: "42",
		Limit:        10,
	})
	// No WHERE when there's no search; only the ORDER BY ensures the pinned
	// row survives the LIMIT.
	want := `SELECT "id" FROM "public"."users" ORDER BY "id"::text = '42' DESC LIMIT 10`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_EmptyCurrentValue(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email"},
		Query:          "alice",
	})
	// No ORDER BY when CurrentValue is empty.
	if strings.Contains(sql, "ORDER BY") {
		t.Fatalf("did not expect ORDER BY: %s", sql)
	}
	// No OR clause for current value either.
	if strings.Contains(sql, ` OR "id"::text =`) {
		t.Fatalf("did not expect current-value OR: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoLimit(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "public",
		Table:    "users",
		FKColumn: "id",
	})
	if strings.Contains(sql, "LIMIT") {
		t.Fatalf("did not expect LIMIT: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EmptySchema(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Table:    "users",
		FKColumn: "id",
		Limit:    1,
	})
	want := `SELECT "id" FROM "users" LIMIT 1`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_FKColumnAlsoInDisplay(t *testing.T) {
	d := NewDialect()
	// User can check the FK column in the display panel. The SELECT list must
	// only mention it once, but search must still hit it.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"id", "email"},
		Query:          "alice",
		Limit:          10,
	})
	// SELECT must dedupe.
	if !strings.HasPrefix(sql, `SELECT "id", "email" FROM`) {
		t.Fatalf("expected deduped SELECT list, got: %s", sql)
	}
	// Search must scan both columns (id was a display column).
	if !strings.Contains(sql, `"id"::text ILIKE '%alice%' ESCAPE '$'`) {
		t.Fatalf("expected id to be searched: %s", sql)
	}
	if !strings.Contains(sql, `"email"::text ILIKE '%alice%' ESCAPE '$'`) {
		t.Fatalf("expected email to be searched: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoDisplayColumnsSearchesFK(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "public",
		Table:    "users",
		FKColumn: "id",
		Query:    "42",
		Limit:    10,
	})
	// With no display columns, the FK column is the only thing to search on.
	if !strings.Contains(sql, `WHERE ("id"::text ILIKE '%42%' ESCAPE '$')`) {
		t.Fatalf("expected FK-column-only search: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesIdentifierQuotes(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         `we"ird`,
		Table:          `ta"ble`,
		FKColumn:       `co"l`,
		DisplayColumns: []string{`d"isp`},
		Limit:          1,
	})
	// Embedded double quotes must double up so the identifier stays a single
	// quoted token; nothing can break out of the identifier quoting.
	want := `SELECT "co""l", "d""isp" FROM "we""ird"."ta""ble" LIMIT 1`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesLiteralQuotes(t *testing.T) {
	d := NewDialect()
	// A search query with a single quote must not be able to terminate the
	// SQL string literal early; '' is the only escape we need under
	// standard_conforming_strings.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"name"},
		Query:          "O'Brien",
		Limit:          10,
	})
	if !strings.Contains(sql, `'%O''Brien%'`) {
		t.Fatalf("expected doubled-quote escape, got: %s", sql)
	}
	// And the literal must still be balanced (no stray apostrophes).
	if strings.Count(sql, "'")%2 != 0 {
		t.Fatalf("unbalanced quotes: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesLikeWildcards(t *testing.T) {
	d := NewDialect()
	// % and _ in the user's search must be escaped so they match literally
	// instead of acting as LIKE wildcards.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "public",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"name"},
		Query:          "100%_off",
		Limit:          10,
	})
	if !strings.Contains(sql, `'%100$%$_off%'`) {
		t.Fatalf("expected %% and _ to be escaped with $, got: %s", sql)
	}
	if !strings.Contains(sql, `ESCAPE '$'`) {
		t.Fatalf("missing ESCAPE clause: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesEscapeCharInQuery(t *testing.T) {
	d := NewDialect()
	// The escape character itself ($) appearing in user input must be
	// escaped, otherwise it would consume the following character.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "public",
		Table:    "users",
		FKColumn: "id",
		Query:    "$money",
		Limit:    1,
	})
	if !strings.Contains(sql, `'%$$money%'`) {
		t.Fatalf("expected $ to be doubled, got: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_WhitespaceQueryIsIgnored(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "public",
		Table:    "users",
		FKColumn: "id",
		Query:    "    ",
		Limit:    1,
	})
	if strings.Contains(sql, "WHERE") {
		t.Fatalf("whitespace-only query should not produce WHERE: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoTrailingSemicolon(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "public",
		Table:    "users",
		FKColumn: "id",
		Limit:    1,
	})
	if strings.HasSuffix(sql, ";") {
		t.Fatalf("statement must not end with ;: %q", sql)
	}
}

func TestBuildForeignKeyLookupSQL_CurrentValueWithQuote(t *testing.T) {
	d := NewDialect()
	// CurrentValue containing a single quote must also be escaped, otherwise
	// a malicious or unusual id value could break out of the literal.
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:       "public",
		Table:        "users",
		FKColumn:     "id",
		CurrentValue: "O'Brien",
		Limit:        1,
	})
	if !strings.Contains(sql, `"id"::text = 'O''Brien'`) {
		t.Fatalf("expected quote-escaped current value, got: %s", sql)
	}
}
