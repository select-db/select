package main

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
)

// KnownCase is one case of a Go case table as run on one dialect, so the finder
// can tell a statement the suite already pins from one nobody has tried.
type KnownCase struct {
	Case     string `json:"case"` // table:name, stable across runs
	Layer    string `json:"layer"`
	Dialect  string `json:"dialect"`
	SQL      string `json:"sql"`
	Expected any    `json:"expected"`
}

// exportDialects mirrors the dialect table in core/references' tests: the
// completion and reference cases are parameterised by the default schema and
// the identifier quote each dialect's tests pass.
var exportDialects = []struct{ name, defaultSchema, quote string }{
	{"postgresql", "public", `"`},
	{"mysql", "public", "`"},
	{"sqlite", "main", `"`},
}

func exportCases(out io.Writer) error {
	encoder := json.NewEncoder(out)
	emit := func(known KnownCase) error { return encoder.Encode(known) }

	for _, d := range exportDialects {
		for _, c := range testutil.PermCasesFor(d.name) {
			if err := emit(KnownCase{
				Case: "perm_data:" + c.Name, Layer: "permission", Dialect: d.name, SQL: c.SQL,
				Expected: map[string]any{"needs": rightNames(c.Needs), "denied": rightNames(c.Denied), "op": c.Op},
			}); err != nil {
				return err
			}
		}

		seeCases := testutil.GetSeeTestCases()
		if d.name != "mysql" {
			seeCases = append(seeCases, testutil.GetSeeCasesPostgreSQLAndSQLite()...)
		}
		for _, c := range seeCases {
			if err := emit(KnownCase{
				Case: "see_data:" + c.Name, Layer: "permission", Dialect: d.name, SQL: c.SQL,
				Expected: map[string]any{"refused": c.Refused, "columns": c.Columns, "masked": c.Masked},
			}); err != nil {
				return err
			}
		}

		completionCases := core.GetCompletionTestCases(d.defaultSchema, d.quote)
		if d.name == "postgresql" {
			completionCases = append(completionCases, core.GetCompletionCasesPostgreSQL(d.defaultSchema, d.quote)...)
		}
		if d.name != "sqlite" {
			completionCases = append(completionCases, core.GetCompletionCasesPostgreSQLAndMySQL(d.defaultSchema, d.quote)...)
		}
		for _, c := range completionCases {
			if err := emit(KnownCase{
				Case: "completion_test_data:" + c.Name, Layer: "completion", Dialect: d.name, SQL: c.SQL,
				Expected: c.Expected,
			}); err != nil {
				return err
			}
		}

		referenceCases := coreRefs.GetReferencesSelectTests(d.defaultSchema)
		referenceCases = append(referenceCases, coreRefs.GetReferencesUpdateTests(d.defaultSchema)...)
		for _, c := range referenceCases {
			if slices.Contains(c.SkipDialects, d.name) {
				continue
			}
			if err := emit(KnownCase{
				Case: "relation_references_test_data:" + c.Name, Layer: "resolution", Dialect: d.name, SQL: c.SQL,
				Expected: map[string]any{"refs": c.ExpectedRefs, "vtabs": c.ExpectedVtabs},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func rightNames(rights []testutil.Right) []string {
	names := make([]string, 0, len(rights))
	for _, right := range rights {
		names = append(names, right.String())
	}
	return names
}
