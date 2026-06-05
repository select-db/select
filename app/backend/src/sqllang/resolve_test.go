package sqllang

import (
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
	ta "github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/postgresql"
)

func pgDialect() core.SQLDialect { return postgresql.NewDialect() }

func testAnalyzer(t *testing.T) core.Analyzer {
	t.Helper()
	execPath, args, ok := resolveAnalyzer()
	if !ok {
		t.Skip("python analyzer not available")
	}
	a := ta.NewAnalyzer(execPath, args...)
	t.Cleanup(a.Close)
	return a
}

func metaWith(defaultSchema string, schemas ...core.Schema) *core.Metadata {
	return &core.Metadata{DefaultSchema: defaultSchema, Schemas: schemas}
}

func mkSchema(name string, tables []core.Table, views []core.Table) core.Schema {
	return core.Schema{Name: name, Tables: tables, Views: views}
}

func mkTable(name string, cols ...string) core.Table {
	t := core.Table{Name: name}
	for _, c := range cols {
		t.Columns = append(t.Columns, core.Column{Name: c})
	}
	return t
}

func TestResolveCore(t *testing.T) {
	cases := []struct {
		name       string
		meta       *core.Metadata
		dialect    core.SQLDialect
		sql        string
		line, col  int
		wantFound  bool
		wantSchema string
		wantRel    string
		wantCol    string
		wantKind   string
	}{
		{
			name: "bare table - schema scan",
			meta: metaWith("public", mkSchema("public", []core.Table{mkTable("orders", "id")}, nil)),
			sql:  "SELECT * FROM orders",
			line: 1, col: 14,
			wantFound: true, wantSchema: "public", wantRel: "orders", wantKind: "table",
		},
		{
			name: "bare table - SQL mixed-case vs lowercase metadata",
			meta: metaWith("public", mkSchema("public", []core.Table{mkTable("customeraccount", "id")}, nil)),
			sql:  "SELECT * FROM CustomerAccount",
			line: 1, col: 14,
			wantFound: true, wantSchema: "public", wantRel: "customeraccount", wantKind: "table",
		},
		{
			name:    "bare table - ref resolves schema",
			meta:    metaWith("zygon", mkSchema("zygon", []core.Table{mkTable("customeraccount", "id")}, nil)),
			dialect: pgDialect(),
			sql:     "SELECT * FROM CustomerAccount",
			line:    1, col: 14,
			wantFound: true, wantSchema: "zygon", wantRel: "customeraccount", wantKind: "table",
		},
		{
			name:    "schema.table navigation",
			meta:    metaWith("public", mkSchema("zygon", []core.Table{mkTable("orders", "id")}, nil)),
			dialect: pgDialect(),
			sql:     "SELECT * FROM zygon.orders",
			line:    1, col: 21,
			wantFound: true, wantSchema: "zygon", wantRel: "orders", wantKind: "table",
		},
		{
			name:    "alias.column navigation",
			meta:    metaWith("public", mkSchema("public", []core.Table{mkTable("orders", "id", "amount")}, nil)),
			dialect: pgDialect(),
			sql:     "SELECT o.amount FROM orders o",
			line:    1, col: 9,
			wantFound: true, wantSchema: "public", wantRel: "orders", wantCol: "amount", wantKind: "column",
		},
		{
			name: "bare view",
			meta: metaWith("public", mkSchema("public", nil, []core.Table{mkTable("active_users")})),
			sql:  "SELECT * FROM active_users",
			line: 1, col: 14,
			wantFound: true, wantSchema: "public", wantRel: "active_users", wantKind: "view",
		},
		{
			name: "no match",
			meta: metaWith("public", mkSchema("public", []core.Table{mkTable("orders")}, nil)),
			sql:  "SELECT * FROM unknown_table",
			line: 1, col: 14,
			wantFound: false,
		},
		// CTE aliased in FROM, qualifier is the alias, not the CTE name.
		{
			name:      "CTE alias column - resolves via alias to physical table",
			meta:      metaWith("public", mkSchema("public", []core.Table{mkTable("t1", "c1")}, nil)),
			dialect:   pgDialect(),
			sql:       "WITH cte AS (SELECT c1 FROM t1) SELECT a.c1 FROM cte a",
			line:      1,
			col:       strings.Index("WITH cte AS (SELECT c1 FROM t1) SELECT a.c1 FROM cte a", ".c1") + 2,
			wantFound: true, wantSchema: "public", wantRel: "t1", wantCol: "c1", wantKind: "column",
		},
	}

	analyzer := testAnalyzer(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := resolveCore(tc.meta, tc.dialect, tc.sql, tc.line, tc.col, analyzer)
			found := obj != nil
			if found != tc.wantFound {
				t.Fatalf("found: got %v want %v", found, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if obj.Schema != tc.wantSchema {
				t.Errorf("Schema: got %q want %q", obj.Schema, tc.wantSchema)
			}
			if obj.Rel != tc.wantRel {
				t.Errorf("Rel: got %q want %q", obj.Rel, tc.wantRel)
			}
			if obj.Col != tc.wantCol {
				t.Errorf("Col: got %q want %q", obj.Col, tc.wantCol)
			}
			if obj.Kind != tc.wantKind {
				t.Errorf("Kind: got %q want %q", obj.Kind, tc.wantKind)
			}
		})
	}
}

func TestBuildNodeID_compositeTableAndView(t *testing.T) {
	const dbID = "db-instance-id"
	meta := metaWith(
		"public",
		mkSchema("public", []core.Table{mkTable("orders")}, []core.Table{mkTable("v_orders")}),
	)

	tabID := buildNodeID(dbID, meta, &resolvedObject{Schema: "public", Rel: "orders", Kind: "table"})
	wantTab := dbID + ":schema:public:table:orders"
	if tabID != wantTab {
		t.Fatalf("table id: got %q want %q", tabID, wantTab)
	}

	viewID := buildNodeID(dbID, meta, &resolvedObject{Schema: "public", Rel: "v_orders", Kind: "view"})
	wantView := dbID + ":schema:public:view:v_orders"
	if viewID != wantView {
		t.Fatalf("view id: got %q want %q", viewID, wantView)
	}
}
