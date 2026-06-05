package core_references_test

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

type dialectInfo struct {
	name          string
	dialect       core.SQLDialect
	defaultSchema string
}

var dialects = []dialectInfo{
	{"postgresql", postgresql.NewDialect(), "public"},
	{"sqlite", sqlite.NewDialect(), "main"},
	{"mysql", mysql.NewDialect(), "public"},
}

func shouldSkip(skipDialects []string, dialectName string) bool {
	for _, s := range skipDialects {
		if s == dialectName {
			return true
		}
	}
	return false
}

func runRefTests(t *testing.T, analyzer core.Analyzer, di dialectInfo, tests []coreRefs.ReferencesTest, checkScope bool) {
	t.Helper()
	meta := coreRefs.GetTestMetadata(di.defaultSchema)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if shouldSkip(tt.SkipDialects, di.name) {
				t.Skipf("not supported in %s", di.name)
			}

			refs, vtabs, err := coreRefs.CollectReferencesFromPython(analyzer, tt.SQL, di.dialect, meta)
			if err != nil {
				t.Fatalf("Python call failed: %v", err)
			}

			if len(refs) != len(tt.ExpectedRefs) {
				t.Errorf("refs: got %d, want %d", len(refs), len(tt.ExpectedRefs))
				for i, r := range refs {
					t.Logf("  ref[%d]: schema=%q table=%q alias=%q scope=[%d,%d] n=%d", i, r.Schema, r.Table, r.Alias, r.ScopeStartPos, r.ScopeEndPos, r.NestingLevel)
				}
				return
			}
			for i, exp := range tt.ExpectedRefs {
				act := refs[i]
				if act.Schema != exp.Schema {
					t.Errorf("ref[%d]: schema %q, want %q", i, act.Schema, exp.Schema)
				}
				if act.Table != exp.Table {
					t.Errorf("ref[%d]: table %q, want %q", i, act.Table, exp.Table)
				}
				if act.Alias != exp.Alias {
					t.Errorf("ref[%d]: alias %q, want %q", i, act.Alias, exp.Alias)
				}
				if checkScope {
					if act.ScopeStartPos != exp.ScopeStartPos {
						t.Errorf("ref[%d] (%s): ScopeStartPos %d, want %d", i, act.Table, act.ScopeStartPos, exp.ScopeStartPos)
					}
					if act.ScopeEndPos != exp.ScopeEndPos {
						t.Errorf("ref[%d] (%s): ScopeEndPos %d, want %d", i, act.Table, act.ScopeEndPos, exp.ScopeEndPos)
					}
					if act.NestingLevel != exp.NestingLevel {
						t.Errorf("ref[%d] (%s): NestingLevel %d, want %d", i, act.Table, act.NestingLevel, exp.NestingLevel)
					}
				}
			}

			if len(vtabs) != len(tt.ExpectedVtabs) {
				t.Errorf("vtabs: got %d, want %d", len(vtabs), len(tt.ExpectedVtabs))
				for i, v := range vtabs {
					t.Logf("  vtab[%d]: table=%q cols=%d scope=[%d,%d] n=%d", i, v.Table, len(v.Columns), v.ScopeStartPos, v.ScopeEndPos, v.NestingLevel)
				}
				return
			}
			for i, exp := range tt.ExpectedVtabs {
				act := vtabs[i]
				if act.Table != exp.Table {
					t.Errorf("vtab[%d]: table %q, want %q", i, act.Table, exp.Table)
				}
				if len(exp.Columns) > 0 {
					if len(act.Columns) != len(exp.Columns) {
						t.Errorf("vtab[%d] %q: %d cols, want %d", i, act.Table, len(act.Columns), len(exp.Columns))
						continue
					}
					for j, ec := range exp.Columns {
						if act.Columns[j].Name != ec.Name {
							t.Errorf("vtab[%d] col[%d]: %q, want %q", i, j, act.Columns[j].Name, ec.Name)
						}
					}
				}
				if checkScope {
					if act.ScopeStartPos != exp.ScopeStartPos {
						t.Errorf("vtab[%d] (%s): ScopeStartPos %d, want %d", i, act.Table, act.ScopeStartPos, exp.ScopeStartPos)
					}
					if act.ScopeEndPos != exp.ScopeEndPos {
						t.Errorf("vtab[%d] (%s): ScopeEndPos %d, want %d", i, act.Table, act.ScopeEndPos, exp.ScopeEndPos)
					}
					if act.NestingLevel != exp.NestingLevel {
						t.Errorf("vtab[%d] (%s): NestingLevel %d, want %d", i, act.Table, act.NestingLevel, exp.NestingLevel)
					}
				}
			}
		})
	}
}

// TestCollectReferencesFromPython validates structural reference collection
// across all dialects using the shared test cases.
func TestCollectReferencesFromPython(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	for _, di := range dialects {
		t.Run(di.name, func(t *testing.T) {
			tests := coreRefs.GetReferencesSelectTests(di.defaultSchema)
			tests = append(tests, coreRefs.GetReferencesUpdateTests(di.defaultSchema)...)
			runRefTests(t, analyzer, di, tests, false)
		})
	}
}

// TestScopePositions validates scope char offsets and nesting levels.
// Runs against postgresql only since scope offsets are dialect-agnostic
// (same Python code) and the expected values are hardcoded for "public" schema.
func TestScopePositions(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	di := dialects[0] // postgresql
	runRefTests(t, analyzer, di, []coreRefs.ReferencesTest{
		{
			Name:         "simple table",
			SQL:          "SELECT * FROM t1",
			ExpectedRefs: []core.RelationRef{{Schema: "public", Table: "t1", ScopeStartPos: 0, ScopeEndPos: -1}},
		},
		{
			Name:         "table with alias",
			SQL:          "SELECT * FROM t1 tt1",
			ExpectedRefs: []core.RelationRef{{Schema: "public", Table: "t1", Alias: "tt1", ScopeStartPos: 0, ScopeEndPos: -1}},
		},
		{
			Name: "CTE",
			SQL:  "WITH cte AS (SELECT c1 FROM t1) SELECT * FROM cte",
			ExpectedRefs: []core.RelationRef{
				{Schema: "public", Table: "t1", ScopeStartPos: 13, ScopeEndPos: 30, NestingLevel: 1},
				{Schema: "", Table: "cte", ScopeStartPos: 32, ScopeEndPos: -1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "cte", ScopeStartPos: 31, ScopeEndPos: -1},
			},
		},
		{
			Name: "join with aliases",
			SQL:  "SELECT * FROM t1 tt1 JOIN t2 tt2 ON tt1.c1 = tt2.c1",
			ExpectedRefs: []core.RelationRef{
				{Schema: "public", Table: "t1", Alias: "tt1", ScopeStartPos: 0, ScopeEndPos: -1},
				{Schema: "public", Table: "t2", Alias: "tt2", ScopeStartPos: 0, ScopeEndPos: -1},
			},
		},
		{
			Name: "subquery with alias",
			SQL:  "SELECT * FROM (SELECT c1 FROM t1) subq",
			ExpectedRefs: []core.RelationRef{
				{Schema: "", Table: "subq", ScopeStartPos: 0, ScopeEndPos: -1},
				{Schema: "public", Table: "t1", ScopeStartPos: 25, ScopeEndPos: 32, NestingLevel: 1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "subq", ScopeStartPos: 0, ScopeEndPos: -1},
			},
		},
		{
			Name: "multiple CTEs",
			SQL:  "WITH cte1 AS (SELECT c1 FROM t1), cte2 AS (SELECT * FROM t2) SELECT * FROM cte1 JOIN cte2 ON cte1.c1 = cte2.c1",
			ExpectedRefs: []core.RelationRef{
				{Schema: "public", Table: "t1", ScopeStartPos: 14, ScopeEndPos: 31, NestingLevel: 1},
				{Schema: "public", Table: "t2", ScopeStartPos: 43, ScopeEndPos: 59, NestingLevel: 1},
				{Schema: "", Table: "cte1", ScopeStartPos: 61, ScopeEndPos: -1},
				{Schema: "", Table: "cte2", ScopeStartPos: 61, ScopeEndPos: -1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "cte1", ScopeStartPos: 32, ScopeEndPos: -1},
				{Table: "cte2", ScopeStartPos: 60, ScopeEndPos: -1},
			},
		},
		{
			Name: "nested subqueries",
			SQL:  "SELECT * FROM (SELECT * FROM (SELECT c1, c2 FROM t2) inner_query WHERE c1 > 10) outer_query",
			ExpectedRefs: []core.RelationRef{
				{Schema: "", Table: "outer_query", ScopeStartPos: 0, ScopeEndPos: -1},
				{Schema: "", Table: "inner_query", ScopeStartPos: 15, ScopeEndPos: 78, NestingLevel: 1},
				{Schema: "public", Table: "t2", ScopeStartPos: 44, ScopeEndPos: 51, NestingLevel: 2},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "outer_query", ScopeStartPos: 0, ScopeEndPos: -1},
				{Table: "inner_query", ScopeStartPos: 15, ScopeEndPos: 78},
			},
		},
		{
			Name: "multi statement with CTE",
			SQL:  "SELECT * FROM t1; WITH cte AS (SELECT * FROM t2) SELECT * FROM cte",
			ExpectedRefs: []core.RelationRef{
				{Schema: "public", Table: "t1", ScopeStartPos: 0, ScopeEndPos: 16},
				{Schema: "public", Table: "t2", ScopeStartPos: 31, ScopeEndPos: 47, NestingLevel: 1},
				{Schema: "", Table: "cte", ScopeStartPos: 49, ScopeEndPos: -1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "cte", ScopeStartPos: 48, ScopeEndPos: -1},
			},
		},
		{
			Name: "recursive CTE",
			SQL:  "WITH RECURSIVE cte AS (SELECT 1 as n UNION ALL SELECT n+1 FROM cte WHERE n < 5) SELECT * FROM cte",
			ExpectedRefs: []core.RelationRef{
				{Schema: "", Table: "cte", ScopeStartPos: 80, ScopeEndPos: -1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "cte", ScopeStartPos: 79, ScopeEndPos: -1},
			},
		},
		{
			Name: "CTE scope end in subquery",
			SQL:  "WITH temp_data AS (SELECT c1, c2 FROM t2) SELECT * FROM temp_data WHERE c1 IN (SELECT c1 FROM temp_data WHERE c2 = 1)",
			ExpectedRefs: []core.RelationRef{
				{Schema: "public", Table: "t2", ScopeStartPos: 19, ScopeEndPos: 40, NestingLevel: 1},
				{Schema: "", Table: "temp_data", ScopeStartPos: 79, ScopeEndPos: 116, NestingLevel: 1},
				{Schema: "", Table: "temp_data", ScopeStartPos: 42, ScopeEndPos: -1},
			},
			ExpectedVtabs: []core.RelationRef{
				{Table: "temp_data", ScopeStartPos: 41, ScopeEndPos: -1},
			},
		},
	}, true)
}
