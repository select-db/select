package main

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
	mysqlcases "github.com/selectDb/dialect/mysql/cases"
	pgcases "github.com/selectDb/dialect/postgresql/cases"
	sqlitecases "github.com/selectDb/dialect/sqlite/cases"
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
var exportDialects = []struct {
	name, defaultSchema, quote string
	// The tables in <dialect>/cases. TestExportReadsEveryDialectTable fails
	// when that package gains a table this list does not name.
	inspect func(defaultSchema string) []core.InspectTestCase
	see     func() []testutil.SeeCase
}{
	{"postgresql", "public", `"`, pgcases.InspectCases, pgcases.SeeCases},
	{"mysql", "public", "`", mysqlcases.InspectCases, mysqlcases.SeeCases},
	{"sqlite", "main", `"`, sqlitecases.InspectCases, sqlitecases.SeeCases},
}

// exporter writes the tables of one dialect, stopping at the first write error.
type exporter struct {
	encoder *json.Encoder
	dialect string
	err     error
}

func exportTable[C any](e *exporter, table, layer string, cases []C, describe func(C) (name, sql string, expected any, skip bool)) {
	for _, c := range cases {
		name, sql, expected, skip := describe(c)
		if skip || e.err != nil {
			continue
		}
		e.err = e.encoder.Encode(KnownCase{Case: table + ":" + name, Layer: layer, Dialect: e.dialect, SQL: sql, Expected: expected})
	}
}

func exportCases(out io.Writer) error {
	encoder := json.NewEncoder(out)
	// The inspectors resolve against the inspect catalog's own default schema.
	inspectSchema := core.GetInspectTestMetadata().DefaultSchema

	for _, d := range exportDialects {
		e := &exporter{encoder: encoder, dialect: d.name}

		exportTable(e, "perm_data", "permission", testutil.PermCasesFor(d.name), func(c testutil.PermCase) (string, string, any, bool) {
			return c.Name, c.SQL, map[string]any{"needs": rightNames(c.Needs), "denied": rightNames(c.Denied), "op": c.Op}, false
		})

		seeCase := func(c testutil.SeeCase) (string, string, any, bool) {
			return c.Name, c.SQL, map[string]any{"refused": c.Refused, "columns": c.Columns, "masked": c.Masked}, false
		}
		sharedSee := testutil.GetSeeTestCases()
		if d.name != "mysql" {
			sharedSee = append(sharedSee, testutil.GetSeeCasesPostgreSQLAndSQLite()...)
		}
		exportTable(e, "see_data", "permission", sharedSee, seeCase)
		exportTable(e, d.name+"/cases.see", "permission", d.see(), seeCase)

		inspectCase := func(c core.InspectTestCase) (string, string, any, bool) {
			return c.Name, c.SQL, map[string]any{"inspect": convertStatements(c.Expected)}, false
		}
		exportTable(e, "inspect_test_data", "permission", core.GetInspectTestCases(inspectSchema), inspectCase)
		exportTable(e, d.name+"/cases.inspect", "permission", d.inspect(inspectSchema), inspectCase)

		completion := core.GetCompletionTestCases(d.defaultSchema, d.quote)
		if d.name == "postgresql" {
			completion = append(completion, core.GetCompletionCasesPostgreSQL(d.defaultSchema, d.quote)...)
		}
		if d.name != "sqlite" {
			completion = append(completion, core.GetCompletionCasesPostgreSQLAndMySQL(d.defaultSchema, d.quote)...)
		}
		exportTable(e, "completion_test_data", "completion", completion, func(c core.CompletionTestCase) (string, string, any, bool) {
			return c.Name, c.SQL, c.Expected, false
		})

		referenceCase := func(c coreRefs.ReferencesTest) (string, string, any, bool) {
			return c.Name, c.SQL, map[string]any{"refs": c.ExpectedRefs, "vtabs": c.ExpectedVtabs}, slices.Contains(c.SkipDialects, d.name)
		}
		references := append(coreRefs.GetReferencesSelectTests(d.defaultSchema), coreRefs.GetReferencesUpdateTests(d.defaultSchema)...)
		exportTable(e, "relation_references_test_data", "resolution", references, referenceCase)
		// Scope offsets are pinned for PostgreSQL's "public" schema only.
		if d.name == "postgresql" {
			exportTable(e, "scope_positions", "resolution", coreRefs.GetScopePositionTests(), referenceCase)
		}

		if e.err != nil {
			return e.err
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
