package core_references_test

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/postgresql"
)

func countColumnRefs(refs []coreRefs.ColumnRef, col string, resolved bool) int {
	n := 0
	for _, r := range refs {
		if r.Column == col && r.Resolved == resolved {
			n++
		}
	}
	return n
}

func countColumnRefsAtLevel(refs []coreRefs.ColumnRef, col string, resolved bool, level int) int {
	n := 0
	for _, r := range refs {
		if r.Column == col && r.Resolved == resolved && r.NestingLevel == level {
			n++
		}
	}
	return n
}

func collectColumnRefsPython(t *testing.T, analyzer core.Analyzer, sql string) []coreRefs.ColumnRef {
	t.Helper()
	d := postgresql.NewDialect()
	meta := coreRefs.GetTestMetadata("public")
	refs, _, err := coreRefs.CollectColumnRefsFromPython(analyzer, sql, d, meta)
	if err != nil {
		t.Fatalf("Python call failed: %v", err)
	}
	return refs
}

func TestCollectColumnRefsFromPython(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	tests := []struct {
		name     string
		sql      string
		assertFn func(t *testing.T, refs []coreRefs.ColumnRef)
	}{
		{
			name: "resolves virtual table columns",
			sql:  "WITH cte AS (SELECT c1 FROM t1) SELECT c1 FROM cte WHERE c1 > 0",
			assertFn: func(t *testing.T, refs []coreRefs.ColumnRef) {
				if countColumnRefs(refs, "c1", true) < 2 {
					t.Fatalf("expected c1 to resolve from virtual table in SELECT and WHERE, got %#v", refs)
				}
			},
		},
		{
			name: "strict current scope no outer fallback",
			sql:  "SELECT c2 FROM t2 WHERE EXISTS (SELECT 1 FROM t1 WHERE c2 > 0)",
			assertFn: func(t *testing.T, refs []coreRefs.ColumnRef) {
				if countColumnRefs(refs, "c2", true) == 0 {
					t.Fatalf("expected outer c2 to resolve, got %#v", refs)
				}
				if countColumnRefs(refs, "c2", false) == 0 {
					t.Fatalf("expected inner c2 to be unresolved with strict current-scope semantics, got %#v", refs)
				}
			},
		},
		{
			name: "order by alias is not unknown column",
			sql:  "SELECT c1 AS alias1 FROM t1 ORDER BY alias1",
			assertFn: func(t *testing.T, refs []coreRefs.ColumnRef) {
				if countColumnRefs(refs, "alias1", false) > 0 {
					t.Fatalf("expected ORDER BY alias1 not to be reported as unknown column, got %#v", refs)
				}
			},
		},
		{
			name: "CTE with $var placeholders, outer qualified refs resolve",
			sql: `WITH cte AS (SELECT id FROM "AppAccount" WHERE id = $x)
				SELECT aa.id FROM cte aa`,
			assertFn: func(t *testing.T, refs []coreRefs.ColumnRef) {
				if countColumnRefsAtLevel(refs, "id", false, 0) > 0 {
					t.Fatalf("expected aa.id to resolve, got: %#v", refs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := collectColumnRefsPython(t, analyzer, tt.sql)
			tt.assertFn(t, refs)
		})
	}
}
