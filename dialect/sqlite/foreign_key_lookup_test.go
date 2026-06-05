package sqlite

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
)

func TestBuildForeignKeyLookupSQL_Basic(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "main",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Limit:          200,
	})
	want := `SELECT "id", "email", "name" FROM "main"."users" LIMIT 200`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_WithSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "main",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email", "name"},
		Query:          "alice",
		Limit:          50,
	})
	want := `SELECT "id", "email", "name" FROM "main"."users" WHERE (LOWER(CAST("email" AS TEXT)) LIKE LOWER('%alice%') ESCAPE '$' OR LOWER(CAST("name" AS TEXT)) LIKE LOWER('%alice%') ESCAPE '$') LIMIT 50`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_PinsCurrentValue(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         "main",
		Table:          "users",
		FKColumn:       "id",
		DisplayColumns: []string{"email"},
		Query:          "x",
		CurrentValue:   "42",
		Limit:          200,
	})
	if !strings.Contains(sql, `OR CAST("id" AS TEXT) = '42'`) {
		t.Fatalf("missing current-value OR clause: %s", sql)
	}
	if !strings.Contains(sql, `ORDER BY CAST("id" AS TEXT) = '42' DESC`) {
		t.Fatalf("missing current-value ORDER BY: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_PinWithoutSearch(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:       "main",
		Table:        "users",
		FKColumn:     "id",
		CurrentValue: "42",
		Limit:        10,
	})
	want := `SELECT "id" FROM "main"."users" ORDER BY CAST("id" AS TEXT) = '42' DESC LIMIT 10`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
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

func TestBuildForeignKeyLookupSQL_EscapesIdentifierQuotes(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         `we"ird`,
		Table:          `ta"ble`,
		FKColumn:       `co"l`,
		DisplayColumns: []string{`d"isp`},
		Limit:          1,
	})
	want := `SELECT "co""l", "d""isp" FROM "we""ird"."ta""ble" LIMIT 1`
	if sql != want {
		t.Fatalf("unexpected sql:\n got: %s\nwant: %s", sql, want)
	}
}

func TestBuildForeignKeyLookupSQL_EscapesLikeWildcards(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "main",
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
		Schema:   "main",
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
		Schema:   "main",
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
		Schema:   "main",
		Table:    "users",
		FKColumn: "id",
	})
	if strings.Contains(sql, "LIMIT") {
		t.Fatalf("did not expect LIMIT: %s", sql)
	}
}

func TestBuildForeignKeyLookupSQL_NoTrailingSemicolon(t *testing.T) {
	d := NewDialect()
	sql := d.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:   "main",
		Table:    "users",
		FKColumn: "id",
		Limit:    1,
	})
	if strings.HasSuffix(sql, ";") {
		t.Fatalf("statement must not end with ;: %q", sql)
	}
}

// Execute the generated SQL against an in-memory SQLite database so we know the
// dialect's escape strategy actually survives the real parser, not just our
// expectations about it. This catches regressions where the produced SQL is
// syntactically valid against a fixture but rejected by the engine.
func TestBuildForeignKeyLookupSQL_RoundtripsThroughSQLite(t *testing.T) {
	// Driver name "sqlite3" matches what the rest of this package registers
	// in its init (see schema_test.go).
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	setup := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, note TEXT)`,
		`INSERT INTO users (id, name, note) VALUES
            (1, 'Alice', '100% off'),
            (2, 'Bob', 'no_underscore'),
            (3, 'O''Brien', 'pa\th'),
            (4, '$money', 'escape test'),
            (5, '"quoted"', 'ident test')`,
	}
	for _, s := range setup {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	d := NewDialect()

	type tc struct {
		name      string
		params    core.ForeignKeyLookupParams
		wantIDs   []int64
		wantFirst int64 // if non-zero, the first row's id must equal this
	}

	tests := []tc{
		{
			name: "wildcard percent matches literally",
			params: core.ForeignKeyLookupParams{
				Schema: "main", Table: "users", FKColumn: "id",
				DisplayColumns: []string{"note"},
				Query:          "100%", Limit: 10,
			},
			wantIDs: []int64{1},
		},
		{
			name: "wildcard underscore matches literally",
			params: core.ForeignKeyLookupParams{
				Schema: "main", Table: "users", FKColumn: "id",
				DisplayColumns: []string{"note"},
				Query:          "no_under", Limit: 10,
			},
			wantIDs: []int64{2},
		},
		{
			name: "single quote in query",
			params: core.ForeignKeyLookupParams{
				Schema: "main", Table: "users", FKColumn: "id",
				DisplayColumns: []string{"name"},
				Query:          "O'Brien", Limit: 10,
			},
			wantIDs: []int64{3},
		},
		{
			name: "escape character in query",
			params: core.ForeignKeyLookupParams{
				Schema: "main", Table: "users", FKColumn: "id",
				DisplayColumns: []string{"name"},
				Query:          "$money", Limit: 10,
			},
			wantIDs: []int64{4},
		},
		{
			name: "current value pinned to top despite filter",
			params: core.ForeignKeyLookupParams{
				Schema: "main", Table: "users", FKColumn: "id",
				DisplayColumns: []string{"name"},
				Query:          "Alice",
				CurrentValue:   "2",
				Limit:          10,
			},
			wantFirst: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := d.BuildForeignKeyLookupSQL(tt.params)
			rows, err := db.Query(query)
			if err != nil {
				t.Fatalf("query failed: %v\nsql: %s", err, query)
			}
			defer rows.Close()

			var got []int64
			for rows.Next() {
				var id int64
				cols, _ := rows.Columns()
				dest := make([]any, len(cols))
				for i := range dest {
					dest[i] = new(any)
				}
				// First column is always the FK column.
				dest[0] = &id
				if err := rows.Scan(dest...); err != nil {
					t.Fatalf("scan: %v", err)
				}
				got = append(got, id)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows.Err: %v", err)
			}

			if tt.wantFirst != 0 {
				if len(got) == 0 || got[0] != tt.wantFirst {
					t.Fatalf("expected first id %d, got %v\nsql: %s", tt.wantFirst, got, query)
				}
				return
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("expected %d rows, got %d (%v)\nsql: %s", len(tt.wantIDs), len(got), got, query)
			}
			for i, want := range tt.wantIDs {
				if got[i] != want {
					t.Fatalf("row %d: expected id %d, got %d\nsql: %s", i, want, got[i], query)
				}
			}
		})
	}
}
