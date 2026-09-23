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
	// identifierQuote is how this dialect spells a quoted identifier. SQLite
	// accepts the double quote as well as brackets, so one character covers it.
	identifierQuote string
}

var dialects = []dialectInfo{
	{"postgresql", postgresql.NewDialect(), "public", `"`},
	{"sqlite", sqlite.NewDialect(), "main", `"`},
	{"mysql", mysql.NewDialect(), "public", "`"},
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
	runRefTests(t, analyzer, di, coreRefs.GetScopePositionTests(), true)
}
