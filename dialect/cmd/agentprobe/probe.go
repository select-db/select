package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/engine"
)

// Case is one input line of a batch. ID is echoed back so a caller can join
// results to what it asked without relying on order.
type Case struct {
	ID      string `json:"id,omitempty"`
	Dialect string `json:"dialect"`
	SQL     string `json:"sql"` // a | marks the caret
}

// Result is everything one case produced. Fields are measurements, never
// expectations: deciding whether a result is wrong is the caller's job.
type Result struct {
	ID              string       `json:"id,omitempty"`
	Dialect         string       `json:"dialect,omitempty"`
	SQL             string       `json:"sql,omitempty"`
	Caret           *Caret       `json:"caret,omitempty"`
	Lint            []Lint       `json:"lint"`
	Completion      []Suggestion `json:"completion,omitempty"`
	CompletionTotal int          `json:"completion_total,omitempty"` // before -completion-limit
	CompletionError string       `json:"completion_error,omitempty"`
	Inspect         []Statement  `json:"inspect"`
	Permissions     Permissions  `json:"permissions"`
	Analyzer        []string     `json:"analyzer,omitempty"`
	Error           string       `json:"error,omitempty"`
}

type Caret struct {
	Line int `json:"line"` // 1-based
	Col  int `json:"col"`  // 0-based
}

type Lint struct {
	Rule      string `json:"rule"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
}

type Suggestion struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Insert string `json:"insert,omitempty"`
}

type Statement struct {
	Op         string      `json:"op"`
	Tables     []string    `json:"tables"`
	Fields     []string    `json:"fields,omitempty"`
	Where      []string    `json:"where,omitempty"`
	Filter     bool        `json:"filter,omitempty"`
	Subqueries []Statement `json:"subqueries,omitempty"`
	Also       []Statement `json:"also,omitempty"`
}

// Permissions is what the checker decided, measured through engine.Inspect so
// the unknown-statement floor applies as it does in production.
//
// Needs is built by starting from a policy granting nothing and adding each
// right the checker names in its refusal until it lets the statement run.
// Converged is false when it never did, which is itself a finding: some right
// the checker asks for cannot be granted.
type Permissions struct {
	Needs     []string  `json:"needs"`
	Converged bool      `json:"converged"`
	Policies  []Verdict `json:"policies"`
}

type Verdict struct {
	Policy  string `json:"policy"`
	Allowed bool   `json:"allowed"`
	Denial  string `json:"denial,omitempty"`
}

// policies brackets each statement the way .claude/dialect/method.md does:
// anything deny_all allows is a total bypass, anything data allows ran without
// the administration right.
var policies = []struct {
	name   string
	rights []testutil.Right
}{
	{"deny_all", nil},
	{"writer", []testutil.Right{{Action: core.ActionInsert}, {Action: core.ActionUpdate}, {Action: core.ActionDelete}}},
	{"data", []testutil.Right{{Action: core.ActionSelect}, {Action: core.ActionInsert}, {Action: core.ActionUpdate}, {Action: core.ActionDelete}}},
	{"manage", []testutil.Right{testutil.Manage}},
}

// maxNeeds bounds the walk that builds Permissions.Needs. No real statement
// asks for this many distinct rights; reaching it means the walk is not
// converging.
const maxNeeds = 64

type prober struct {
	analyzer *tokenanalyzer.Analyzer
	meta     core.Metadata
	raw      bool
	// completionLimit keeps a batch readable: an empty WHERE offers every
	// function the dialect has, and the first entries are the ranked ones.
	completionLimit int
}

func (p prober) probe(probeCase Case) Result {
	result := Result{ID: probeCase.ID, Dialect: probeCase.Dialect, SQL: probeCase.SQL}
	dialect := engine.GetDialect(probeCase.Dialect)
	if dialect == nil {
		result.Error = fmt.Sprintf("unknown dialect %q (want postgresql, mysql or sqlite)", probeCase.Dialect)
		return result
	}
	dialect.SetAnalyzer(p.analyzer)

	sqlText, caretLine, caretCol := cutCaret(probeCase.SQL)
	if caretLine > 0 {
		result.Caret = &Caret{Line: caretLine, Col: caretCol}
	}

	result.Lint = p.lint(dialect, sqlText)
	if result.Caret != nil {
		result.Completion, result.CompletionError = p.complete(dialect, sqlText, *result.Caret)
		result.CompletionTotal = len(result.Completion)
		if p.completionLimit > 0 && len(result.Completion) > p.completionLimit {
			result.Completion = result.Completion[:p.completionLimit]
		}
	}
	statements := engine.Inspect(dialect, &p.meta, sqlText)
	result.Inspect = convertStatements(statements)
	result.Permissions = measurePermissions(statements)
	if p.raw {
		result.Analyzer = p.analyzerView(dialect, sqlText, result.Caret)
	}
	return result
}

func (p prober) lint(dialect core.SQLDialect, sqlText string) []Lint {
	runner := &tokenanalyzer.LintRunner{Analyzer: p.analyzer}
	diagnostics := runner.Run(sqlText, dialect, p.meta, tokenanalyzer.ResolvedLintConfig{})
	lints := make([]Lint, 0, len(diagnostics))
	for _, diag := range diagnostics {
		lints = append(lints, Lint{
			Rule:      diag.RuleID,
			Severity:  diag.Severity.String(),
			Message:   diag.Message,
			StartLine: diag.StartLine,
			StartCol:  diag.StartCol,
			EndLine:   diag.EndLine,
			EndCol:    diag.EndCol,
		})
	}
	return lints
}

func (p prober) complete(dialect core.SQLDialect, sqlText string, caret Caret) ([]Suggestion, string) {
	candidates, err := dialect.Complete(context.Background(), sqlText, caret.Line, caret.Col, p.meta)
	if err != nil {
		return nil, err.Error()
	}
	suggestions := make([]Suggestion, 0, len(candidates))
	for _, c := range candidates {
		suggestion := Suggestion{Type: c.Type.String(), Text: c.Text}
		if c.InsertText != c.Text {
			suggestion.Insert = c.InsertText
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, ""
}

func convertStatements(statements []core.InspectStatement) []Statement {
	converted := make([]Statement, 0, len(statements))
	for _, stmt := range statements {
		tables := make([]string, 0, len(stmt.Tables))
		for _, t := range stmt.Tables {
			tables = append(tables, qualify(t.Schema, t.Name))
		}
		converted = append(converted, Statement{
			Op:         string(stmt.Operation),
			Tables:     tables,
			Fields:     fieldNames(stmt.Fields),
			Where:      fieldNames(stmt.Where),
			Filter:     stmt.Filter,
			Subqueries: convertStatements(stmt.Subqueries),
			Also:       convertStatements(stmt.Also),
		})
	}
	return converted
}

func measurePermissions(statements []core.InspectStatement) Permissions {
	var measured Permissions
	for _, policy := range policies {
		verdict := Verdict{Policy: policy.name, Allowed: true}
		if err := check(statements, policy.rights); err != nil {
			verdict.Allowed, verdict.Denial = false, err.Error()
		}
		measured.Policies = append(measured.Policies, verdict)
	}

	var held []testutil.Right
	for len(held) < maxNeeds {
		err := check(statements, held)
		if err == nil {
			measured.Converged = true
			break
		}
		var denied *core.PermissionDeniedError
		if !errors.As(err, &denied) {
			break
		}
		right := testutil.Right{Action: denied.Action, Schema: denied.Schema, Table: denied.Table, Column: denied.Column}
		// Manage is held on the connection, so a refusal that names the table
		// it was checked against is only satisfied by the connection-wide grant.
		if denied.Action == core.ActionManage {
			right = testutil.Manage
		}
		if slices.Contains(held, right) {
			break
		}
		held = append(held, right)
	}
	measured.Needs = make([]string, 0, len(held))
	for _, right := range held {
		measured.Needs = append(measured.Needs, right.String())
	}
	return measured
}

func check(statements []core.InspectStatement, rights []testutil.Right) error {
	return core.CheckQueryPermissions(statements, testutil.TestDatasourceID, testutil.PermGranting(rights...))
}

func fieldNames(fields []core.InspectField) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, qualify(f.Table, f.Name))
	}
	return names
}

func qualify(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// analyzerView reports what the analyzer answered, which separates a sqlglot
// parsing bug from a Go bug in the layer consuming it. It goes through the same
// core/references calls the dialects use, so the probe cannot drift from the
// requests the real completion path sends.
func (p prober) analyzerView(dialect core.SQLDialect, sqlText string, caret *Caret) []string {
	var lines []string
	var opts []any
	if caret != nil {
		opts = append(opts, &coreRefs.CompletionCaret{Line: caret.Line, Col: caret.Col})
	}

	relations, virtualTables, err := coreRefs.CollectReferencesFromPython(p.analyzer, sqlText, dialect, p.meta, opts...)
	if err != nil {
		lines = append(lines, fmt.Sprintf("references: FAILED: %v", err))
	} else {
		for _, r := range relations {
			lines = append(lines, fmt.Sprintf("relation   %s alias=%q virtual=%t level=%d at %d:%d",
				qualify(r.Schema, r.Table), r.Alias, r.IsVirtual, r.NestingLevel, r.Line, r.Col))
		}
		for _, v := range virtualTables {
			lines = append(lines, fmt.Sprintf("cte        %s columns=%d", v.Table, len(v.Columns)))
		}
	}

	columnRefs, aliases, err := coreRefs.CollectColumnRefsFromPython(p.analyzer, sqlText, dialect, p.meta, opts...)
	if err != nil {
		lines = append(lines, fmt.Sprintf("column refs: FAILED: %v", err))
	} else {
		for _, c := range columnRefs {
			lines = append(lines, fmt.Sprintf("column ref %s qualified=%t resolved=%t level=%d at %d:%d",
				c.Column, c.Qualified, c.Resolved, c.NestingLevel, c.Line, c.Col))
		}
		for _, a := range aliases {
			lines = append(lines, fmt.Sprintf("alias      %s", a.Alias))
		}
	}

	if caret == nil {
		return lines
	}
	completionCtx, err := coreRefs.ParseCompletionContextFromPython(p.analyzer, sqlText, dialect, caret.Line, caret.Col, p.meta)
	if err != nil {
		return append(lines, fmt.Sprintf("context: FAILED: %v", err))
	}
	precedingColumn := "none"
	if completionCtx.PrecedingColumn != nil {
		precedingColumn = completionCtx.PrecedingColumn.Name
	}
	return append(lines, fmt.Sprintf("context    targets=%d parts=%v afterDot=%t table=%q preceding=%s valuePos=%t",
		completionCtx.Targets, completionCtx.Parts, completionCtx.CaretAfterDot,
		completionCtx.TargetTable, precedingColumn, completionCtx.ValuePosition))
}
