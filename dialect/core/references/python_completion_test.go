package core_references_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/postgresql"
)

func TestParseCompletionContextFromPython(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	// These cases are spelled the same in every dialect, so one stands in for
	// all three. Per-dialect quoting and settings syntax live in the dialect
	// packages, next to the code that defines them.
	dialect := postgresql.NewDialect()
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
		wantColumnList   string
	}{
		{name: "SELECT without FROM", sql: "SELECT | ", wantTargets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "SELECT with FROM", sql: "SELECT | FROM t1", wantTargets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "qualified table column", sql: "SELECT t1.| FROM t1",
			wantTargets: core.CompletionTargetColumn, wantParts: []string{"t1"}, wantCaretDot: true, wantTargetTable: "t1"},
		{name: "FROM clause", sql: "SELECT * FROM |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "WHERE clause", sql: "SELECT * FROM t1 WHERE |", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "ORDER BY clause", sql: "SELECT * FROM t1 ORDER BY |", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "GROUP BY clause", sql: "SELECT * FROM t1 GROUP BY |", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "HAVING clause", sql: "SELECT * FROM t1 GROUP BY c1 HAVING |", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "JOIN clause", sql: "SELECT * FROM t1 JOIN |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "ON clause", sql: "SELECT * FROM t1 JOIN t2 ON |", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "UPDATE SET", sql: "UPDATE t1 SET |", wantTargets: core.CompletionTargetColumn},
		{name: "INSERT INTO", sql: "INSERT INTO |", wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "schema qualified", sql: "SELECT public.| FROM public.t1",
			wantTargets: core.CompletionTargetTable, wantParts: []string{"public"}, wantCaretDot: true, wantSchemaFilter: "public"},
		{name: "schema.table qualified", sql: "SELECT public.t1.| FROM public.t1",
			wantTargets: core.CompletionTargetColumn, wantParts: []string{"public", "t1"}, wantCaretDot: true, wantSchemaFilter: "public", wantTargetTable: "t1"},
		{name: "subquery in WHERE", sql: "SELECT * FROM t1 WHERE c1 IN (SELECT | FROM t2)", wantTargets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "INSERT column list", sql: "INSERT INTO t1 (|", wantTargets: core.CompletionTargetColumn, wantColumnList: "t1"},
		{name: "operator context", sql: "SELECT * FROM t1 WHERE c1 |", wantTargets: core.CompletionTargetOperator},
		{name: "enum value equality", sql: "SELECT * FROM t1 WHERE c1 = '|'", wantTargets: core.CompletionTargetEnumValue},
		{name: "enum value in list", sql: "SELECT * FROM t1 WHERE c1 IN ('|')", wantTargets: core.CompletionTargetEnumValue},
		{name: "a scalar subquery after a comparison", sql: "SELECT * FROM t1 WHERE c1 = (|",
			wantTargets: core.CompletionTargetSchemaAndTableAll},
		{name: "a clause word used as a column name", sql: "UPDATE t1 SET limit = |",
			wantTargets: core.CompletionTargetColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "enum value update set", sql: "UPDATE t1 SET c1 = '|'", wantTargets: core.CompletionTargetEnumValue},
		{name: "enum value typing a prefix", sql: "SELECT * FROM t1 WHERE c1 = ac|",
			wantTargets: core.CompletionTargetEnumValue | core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "enum value in list typing a prefix", sql: "SELECT * FROM t1 WHERE c1 IN (ac|",
			wantTargets: core.CompletionTargetEnumValue | core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "enum value update set typing a prefix", sql: "UPDATE t1 SET c1 = ac|",
			wantTargets: core.CompletionTargetEnumValue | core.CompletionTargetColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "SELECT mid-word typing", sql: "SELECT cus| FROM t1", wantTargets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "WHERE mid-word typing", sql: "SELECT * FROM t1 WHERE cus|", wantTargets: core.CompletionTargetTableAndColumn | core.CompletionTargetFunction | core.CompletionTargetKeyword},
		{name: "SELECT empty quoted identifier", sql: "SELECT \"|\" FROM t1", wantTargets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, caretCharPos := removeCaret(tt.sql)
			caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

			ctx, err := coreRefs.ParseCompletionContextFromPython(analyzer, text, dialect, caretLine, caretOffset, meta)
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
			if ctx.ColumnListRelation != tt.wantColumnList {
				t.Errorf("ColumnListRelation = %q, want %q", ctx.ColumnListRelation, tt.wantColumnList)
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
			testCases := core.GetCompletionTestCases(di.defaultSchema, di.identifierQuote)
			if di.name == "postgresql" {
				testCases = append(testCases,
					core.GetCompletionCasesPostgreSQL(di.defaultSchema, di.identifierQuote)...)
			}
			if di.name == "postgresql" || di.name == "mysql" {
				testCases = append(testCases,
					core.GetCompletionCasesPostgreSQLAndMySQL(di.defaultSchema, di.identifierQuote)...)
			}

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

// TestCompletionFunctions runs the cases naming which carets take a call. The
// builtins are the dialect's hundreds, so the cases assert that they are
// offered and that one the dialect declares is among them.
// TestEveryKeywordIsReachable fails when a dialect declares a word no caret
// can offer, which would be vocabulary nobody ever sees.
func TestEveryKeywordIsReachable(t *testing.T) {
	for _, di := range dialects {
		t.Run(di.name, func(t *testing.T) {
			if stranded := core.UnreachableKeywords(di.dialect); len(stranded) > 0 {
				t.Errorf("no caret offers %v", stranded)
			}
		})
	}
}

// TestNothingChangesWhileAnItemIsUnfinished runs every completion case twice:
// once as written, once with an unfinished item standing after the caret. An
// item nobody has finished names no relation and defines no alias, so it can
// change nothing, and each case answering differently is a hole.
//
// Six hand written cases would only cover the shapes somebody thought of.
// This covers whatever the table holds, which is what the method was missing.
func TestNothingChangesWhileAnItemIsUnfinished(t *testing.T) {
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

			complete := func(t *testing.T, sql string, caretCharPos int) []core.Candidate {
				t.Helper()
				caretLine, caretOffset := charPosToLineCol(sql, caretCharPos)
				got, err := di.dialect.Complete(context.Background(), sql, caretLine, caretOffset, meta)
				if err != nil {
					t.Fatalf("complete: %v", err)
				}
				return got
			}

			for _, tc := range core.GetCompletionTestCases(di.defaultSchema, di.identifierQuote) {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					if !inASelectList(text, caretCharPos) {
						// Anywhere else the added name reads as something the
						// statement wanted: a relation in a FROM, the target
						// of an INSERT. Only in a list of items is a missing
						// separator what the writer has done wrong.
						t.Skip("not a list of items")
					}
					want := complete(t, text, caretCharPos)

					// A second item on a line of its own with no comma before
					// it, which is what a writer leaves behind mid-edit.
					unfinished := text[:caretCharPos] + "\n  zz.zz\n" + text[caretCharPos:]
					got := complete(t, unfinished, caretCharPos)

					if len(got) != len(want) {
						t.Fatalf("%d candidates with an unfinished item after the caret, %d without",
							len(got), len(want))
					}
					for i := range got {
						if got[i].Text != want[i].Text || got[i].Type != want[i].Type {
							t.Errorf("candidate %d = %q, want %q", i, got[i].Text, want[i].Text)
						}
					}
				})
			}
		})
	}
}

// inASelectList reports the caret as standing among a SELECT's items, where
// the separator between two of them is a comma and leaving it out is the
// ordinary state of a buffer being typed.
func inASelectList(sql string, caret int) bool {
	before := strings.ToUpper(sql[:caret])
	if semi := strings.LastIndex(before, ";"); semi >= 0 {
		before = before[semi+1:]
	}
	start := strings.LastIndex(before, "SELECT")
	if start < 0 {
		return false
	}
	for _, clause := range []string{"FROM", "WHERE", "GROUP BY", "ORDER BY", "HAVING", "(", ")"} {
		if strings.Contains(before[start:], clause) {
			return false
		}
	}
	return true
}

// TestCompletionWhileEditing runs the carets that still have text after them,
// which every caret has until the writer stops typing. A caret at the end of
// the buffer exercises the token walk alone; only these reach the parser with
// a statement it cannot read.
func TestCompletionWhileEditing(t *testing.T) {
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

			for _, tc := range core.GetCompletionEditingCases() {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					var names []string
					for _, c := range got {
						switch c.Type {
						case core.CandidateTypeColumn, core.CandidateTypeTable,
							core.CandidateTypeView, core.CandidateTypeForeignTable,
							core.CandidateTypeMaterializedView:
							names = append(names, c.Text)
						}
					}
					if !slices.Equal(names, tc.Expected) {
						t.Errorf("names = %v, want %v", names, tc.Expected)
					}
				})
			}
		})
	}
}

// TestCompletionTypes runs the carets where a type name stands. The types are
// the database's own, so the fixture's list is the whole answer.
func TestCompletionTypes(t *testing.T) {
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

			for _, tc := range core.GetCompletionTypeCases() {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					var names []string
					for _, c := range got {
						if c.Type == core.CandidateTypeType {
							names = append(names, c.Text)
						}
					}
					if !slices.Equal(names, tc.Expected) {
						t.Errorf("types = %v, want %v", names, tc.Expected)
					}
				})
			}
		})
	}
}

func TestCompletionFunctions(t *testing.T) {
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
			builtins := di.dialect.GetBuiltinFunctions()
			if len(builtins) == 0 {
				t.Fatalf("%s declares no builtin function", di.name)
			}

			for _, tc := range core.GetCompletionFunctionCases() {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					offered := map[string]bool{}
					for _, c := range got {
						if c.Type == core.CandidateTypeFunction {
							offered[c.Text] = true
						}
					}
					if tc.Offered && !offered[builtins[0]] {
						t.Errorf("no call offered; %q is not among %d candidates", builtins[0], len(got))
					}
					if !tc.Offered && len(offered) > 0 {
						t.Errorf("%d calls offered where a column is only being named", len(offered))
					}
				})
			}
		})
	}
}

// TestCompletionClauses runs the cases naming which group of words each caret
// allows. The words are the dialect's own, so the rule is stated once.
func TestCompletionClauses(t *testing.T) {
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

			for _, tc := range core.GetCompletionClauseCases() {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					var words []string
					for _, c := range got {
						if c.Type == core.CandidateTypeKeyword {
							words = append(words, c.Text)
						}
					}
					want := core.KeywordsOfGroup(di.dialect, tc.Group)
					if tc.Group == "" {
						want = nil
					}
					if !slices.Equal(words, want) {
						t.Errorf("keywords = %v, want %v", words, want)
					}
				})
			}
		})
	}
}

// TestCompletionKeywords runs the keyword cases across all dialects. The words
// are the dialect's own, so one table covers three vocabularies.
func TestCompletionKeywords(t *testing.T) {
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
			openers := core.KeywordsOfGroup(di.dialect, "statement")
			if len(openers) == 0 {
				t.Fatalf("%s declares no word a statement can open with", di.name)
			}

			for _, tc := range core.GetCompletionKeywordCases(openers) {
				t.Run(tc.Name, func(t *testing.T) {
					text, caretCharPos := removeCaret(tc.SQL)
					caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

					got, err := di.dialect.Complete(context.Background(), text, caretLine, caretOffset, meta)
					if err != nil {
						t.Fatalf("complete: %v", err)
					}

					var words []string
					for _, c := range got {
						if c.Type == core.CandidateTypeKeyword {
							words = append(words, c.Text)
						}
					}
					if !slices.Equal(words, tc.Expected) {
						t.Errorf("keywords = %v, want %v", words, tc.Expected)
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
