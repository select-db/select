package core_references_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
)

func TestParseCompletionContextFromPython(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	meta := core.GetCompletionTestMetadata()
	meta.DefaultSchema = "public"
	if len(meta.Schemas) > 0 {
		meta.Schemas[0].Name = "public"
	}

	tests := []struct {
		name             string
		sql              string
		wantTargets      core.CompletionTarget
		wantSchemaFilter string
		wantTargetTable  string
		wantParts        []string
		wantCaretDot     bool
		wantInsert       string
	}{
		{name: "SELECT without FROM", sql: "SELECT | ", wantTargets: core.CompletionTargetAll},
		{name: "SELECT with FROM", sql: "SELECT | FROM t1", wantTargets: core.CompletionTargetAll},
		{name: "qualified table column", sql: "SELECT t1.| FROM t1",
			wantTargets: core.CompletionTargetColumn, wantParts: []string{"t1"}, wantCaretDot: true, wantTargetTable: "t1"},
		{name: "FROM clause", sql: "SELECT * FROM |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "WHERE clause", sql: "SELECT * FROM t1 WHERE |", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "ORDER BY clause", sql: "SELECT * FROM t1 ORDER BY |", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "GROUP BY clause", sql: "SELECT * FROM t1 GROUP BY |", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "HAVING clause", sql: "SELECT * FROM t1 GROUP BY c1 HAVING |", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "JOIN clause", sql: "SELECT * FROM t1 JOIN |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "ON clause", sql: "SELECT * FROM t1 JOIN t2 ON |", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "UPDATE SET", sql: "UPDATE t1 SET |", wantTargets: core.CompletionTargetColumn},
		{name: "INSERT INTO", sql: "INSERT INTO |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "schema qualified", sql: "SELECT public.| FROM public.t1",
			wantTargets: core.CompletionTargetTable, wantParts: []string{"public"}, wantCaretDot: true, wantSchemaFilter: "public"},
		{name: "schema.table qualified", sql: "SELECT public.t1.| FROM public.t1",
			wantTargets: core.CompletionTargetColumn, wantParts: []string{"public", "t1"}, wantCaretDot: true, wantSchemaFilter: "public", wantTargetTable: "t1"},
		{name: "subquery in WHERE", sql: "SELECT * FROM t1 WHERE c1 IN (SELECT | FROM t2)", wantTargets: core.CompletionTargetAll},
		{name: "INSERT column list", sql: "INSERT INTO t1 (|", wantTargets: core.CompletionTargetColumn, wantInsert: "t1"},
		{name: "operator context", sql: "SELECT * FROM t1 WHERE c1 |", wantTargets: core.CompletionTargetOperator},
		{name: "enum value equality", sql: "SELECT * FROM t1 WHERE c1 = '|'", wantTargets: core.CompletionTargetEnumValue},
		{name: "enum value in list", sql: "SELECT * FROM t1 WHERE c1 IN ('|')", wantTargets: core.CompletionTargetEnumValue},
		{name: "enum value update set", sql: "UPDATE t1 SET c1 = '|'", wantTargets: core.CompletionTargetEnumValue},
		{name: "SELECT mid-word typing", sql: "SELECT cus| FROM t1", wantTargets: core.CompletionTargetAll},
		{name: "WHERE mid-word typing", sql: "SELECT * FROM t1 WHERE cus|", wantTargets: core.CompletionTargetTableAndColumn},
		{name: "SELECT empty quoted identifier", sql: "SELECT \"|\" FROM t1", wantTargets: core.CompletionTargetAll},
		{name: "MySQL @@ system variable", sql: "SELECT @@|", wantTargets: core.CompletionTargetSetting},
		{name: "MySQL @@ partial", sql: "SELECT @@vers|", wantTargets: core.CompletionTargetSetting},
		{name: "Postgres SHOW", sql: "SHOW |", wantTargets: core.CompletionTargetSetting},
		{name: "Postgres SHOW partial", sql: "SHOW time|", wantTargets: core.CompletionTargetSetting},
		{name: "SQLite PRAGMA", sql: "PRAGMA |", wantTargets: core.CompletionTargetSetting},
		{name: "Single @ user variable does not trigger Setting", sql: "SELECT @my|", wantTargets: core.CompletionTargetAll},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, caretCharPos := removeCaret(tt.sql)
			caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

			ctx, err := coreRefs.ParseCompletionContextFromPython(analyzer, text, caretLine, caretOffset, meta)
			if err != nil {
				t.Fatalf("Python call failed: %v", err)
			}
			if ctx.Targets != tt.wantTargets {
				t.Errorf("Targets = %d, want %d", ctx.Targets, tt.wantTargets)
			}
			if ctx.SchemaFilter != tt.wantSchemaFilter {
				t.Errorf("SchemaFilter = %q, want %q", ctx.SchemaFilter, tt.wantSchemaFilter)
			}
			if ctx.TargetTable != tt.wantTargetTable {
				t.Errorf("TargetTable = %q, want %q", ctx.TargetTable, tt.wantTargetTable)
			}
			if ctx.CaretAfterDot != tt.wantCaretDot {
				t.Errorf("CaretAfterDot = %v, want %v", ctx.CaretAfterDot, tt.wantCaretDot)
			}
			if ctx.InsertTargetTable != tt.wantInsert {
				t.Errorf("InsertTargetTable = %q, want %q", ctx.InsertTargetTable, tt.wantInsert)
			}
			if tt.wantParts != nil && len(ctx.Parts) != len(tt.wantParts) {
				t.Errorf("Parts = %v, want %v", ctx.Parts, tt.wantParts)
			}
		})
	}
}

// TestCompletion runs the shared end-to-end completion test cases across all dialects.
func TestCompletion(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	for _, di := range dialects {
		t.Run(di.name, func(t *testing.T) {
			di.dialect.SetAnalyzer(analyzer)

			meta := core.GetCompletionTestMetadata()
			meta.DefaultSchema = di.defaultSchema
			if len(meta.Schemas) > 0 {
				meta.Schemas[0].Name = di.defaultSchema
			}
			testCases := core.GetCompletionTestCases(di.defaultSchema)

			for _, tc := range testCases {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					filtered := filterCandidates(got)
					filtered = core.DeduplicateAndSort(filtered)

					expected := make([]core.Candidate, 0, len(tc.Expected))
					for _, exp := range tc.Expected {
						expected = append(expected, core.Candidate{Type: exp.Type, Text: exp.Text})
					}

					expectedMap := candidateMap(expected)
					gotMap := candidateMap(filtered)

					for key, exp := range expectedMap {
						if _, found := gotMap[key]; !found {
							t.Errorf("Missing: Type=%d, Text=%s", exp.Type, exp.Text)
						}
					}
					if len(expected) > 0 {
						for key, cand := range gotMap {
							if _, found := expectedMap[key]; !found {
								t.Errorf("Unexpected: Type=%d, Text=%s", cand.Type, cand.Text)
							}
						}
					}
					if len(filtered) != len(expected) {
						t.Errorf("got %d candidates, want %d", len(filtered), len(expected))
					}
				})
			}
		})
	}
}

func removeCaret(s string) (string, int) {
	for i, c := range s {
		if c == '|' {
			return s[:i] + s[i+1:], i
		}
	}
	return s, -1
}

func charPosToLineCol(text string, charPos int) (int, int) {
	lines := strings.Split(text, "\n")
	caretLine := 1
	caretOffset := charPos
	for i, line := range lines {
		if caretOffset <= len(line) {
			caretLine = i + 1
			break
		}
		caretOffset -= len(line) + 1
	}
	return caretLine, caretOffset
}

func filterCandidates(candidates []core.Candidate) []core.Candidate {
	var filtered []core.Candidate
	for _, c := range candidates {
		if c.Type == core.CandidateTypeKeyword || c.Type == core.CandidateTypeFunction || c.Type == core.CandidateTypeType {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func candidateMap(candidates []core.Candidate) map[string]core.Candidate {
	m := make(map[string]core.Candidate)
	for _, c := range candidates {
		m[fmt.Sprintf("%d:%s", c.Type, c.Text)] = c
	}
	return m
}
