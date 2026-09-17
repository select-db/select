// Command sqlprobe runs one SQL string, against one catalog, through every
// dialect-level language feature at once and prints what each produced. It is
// the isolation step for a suspected completion or lint bug: shrink the case
// until exactly one layer is wrong, then write the regression test against that
// layer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/engine"
)

func main() {
	dialectName := flag.String("dialect", "postgresql", "postgresql, mysql or sqlite")
	sqlInput := flag.String("sql", "", "SQL to probe, or @file. A | marks the caret")
	metaPath := flag.String("meta", "", "core.Metadata JSON file")
	showRaw := flag.Bool("raw", false, "also dump what the analyzer returned")
	flag.Parse()

	if err := run(*dialectName, *sqlInput, *metaPath, *showRaw); err != nil {
		fmt.Fprintf(os.Stderr, "sqlprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(dialectName, sqlInput, metaPath string, showRaw bool) error {
	if strings.TrimSpace(sqlInput) == "" {
		return fmt.Errorf("-sql is required")
	}
	if metaPath == "" {
		return fmt.Errorf("-meta is required")
	}

	d := engine.GetDialect(dialectName)
	if d == nil {
		return fmt.Errorf("unknown dialect %q (want postgresql, mysql or sqlite)", dialectName)
	}

	sqlText, err := readArg(sqlInput)
	if err != nil {
		return fmt.Errorf("reading -sql: %w", err)
	}
	meta, err := loadMetadata(metaPath)
	if err != nil {
		return err
	}

	pythonPath, script, ok := tokenanalyzer.FindDevAnalyzer()
	if !ok {
		return fmt.Errorf("python venv not found; run `uv sync` in dialect/core/tokenanalyzer/python")
	}
	analyzer := tokenanalyzer.NewAnalyzer(pythonPath, script)
	defer analyzer.Close()
	d.SetAnalyzer(analyzer)

	sqlText, caretLine, caretCol := cutCaret(sqlText)
	p := probe{
		analyzer:  analyzer,
		dialect:   d,
		meta:      meta,
		sql:       sqlText,
		caretLine: caretLine,
		caretCol:  caretCol,
	}

	fmt.Printf("== dialect %s   default schema %s\n", d.Name(), core.GetDefaultSchema(meta))
	p.printSQL()
	p.printLint()
	if p.hasCaret() {
		p.printCompletion()
	}
	p.printInspect()
	if showRaw {
		p.printAnalyzerView()
	}
	return nil
}

// probe is one case: a dialect, a catalog, a statement, and where the caret sits
// in it. Fixed once run builds it, so the printers take no arguments.
type probe struct {
	analyzer  *tokenanalyzer.Analyzer
	dialect   core.SQLDialect
	meta      core.Metadata
	sql       string
	caretLine int // 1-based, 0 when the SQL carried no caret marker
	caretCol  int // 0-based
}

func (p probe) hasCaret() bool { return p.caretLine > 0 }

func loadMetadata(metaPath string) (core.Metadata, error) {
	if metaPath == "" {
		return core.Metadata{}, fmt.Errorf("-meta is required")
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return core.Metadata{}, fmt.Errorf("reading -meta: %w", err)
	}
	var meta core.Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return core.Metadata{}, fmt.Errorf("parsing -meta: %w", err)
	}
	return meta, nil
}

// readArg returns the literal value, or the file contents when it starts with @.
func readArg(value string) (string, error) {
	path, isFile := strings.CutPrefix(value, "@")
	if !isFile {
		return value, nil
	}
	raw, err := os.ReadFile(path)
	return string(raw), err
}

// cutCaret removes the first | and reports where it was, as the 1-based line
// and 0-based column the dialect layer expects. Returns line 0 when absent.
func cutCaret(sql string) (string, int, int) {
	idx := strings.Index(sql, "|")
	if idx == -1 {
		return sql, 0, 0
	}
	before := sql[:idx]
	line := strings.Count(before, "\n") + 1
	col := len(before) - (strings.LastIndex(before, "\n") + 1)
	return before + sql[idx+1:], line, col
}

func (p probe) printSQL() {
	fmt.Println("\n== sql")
	for i, line := range strings.Split(p.sql, "\n") {
		fmt.Printf("  %2d | %s\n", i+1, line)
		if i+1 == p.caretLine {
			fmt.Printf("     | %s^ caret %d:%d\n", strings.Repeat(" ", p.caretCol), p.caretLine, p.caretCol)
		}
	}
}

func (p probe) printLint() {
	runner := &tokenanalyzer.LintRunner{Analyzer: p.analyzer}
	diagnostics := runner.Run(p.sql, p.dialect, p.meta, tokenanalyzer.ResolvedLintConfig{})

	fmt.Printf("\n== lint (%d)\n", len(diagnostics))
	for _, diag := range diagnostics {
		fmt.Printf("  %-7s %-28s %d:%d-%d:%d  %s\n",
			diag.Severity, diag.RuleID,
			diag.StartLine, diag.StartCol, diag.EndLine, diag.EndCol,
			diag.Message)
	}
}

func (p probe) printCompletion() {
	candidates, err := p.dialect.Complete(context.Background(), p.sql, p.caretLine, p.caretCol, p.meta)
	if err != nil {
		fmt.Printf("\n== complete: FAILED: %v\n", err)
		return
	}

	byType := map[core.CandidateType][]string{}
	for _, c := range candidates {
		label := c.Text
		if c.InsertText != "" && c.InsertText != c.Text {
			label += " -> " + c.InsertText
		}
		byType[c.Type] = append(byType[c.Type], label)
	}

	fmt.Printf("\n== complete @ %d:%d (%d)\n", p.caretLine, p.caretCol, len(candidates))
	for t := core.CandidateTypeKeyword; t <= core.CandidateTypeSetting; t++ {
		if labels := byType[t]; len(labels) > 0 {
			fmt.Printf("  %-17s %s\n", t, strings.Join(labels, ", "))
		}
	}
}

func (p probe) printInspect() {
	statements := p.dialect.Inspect(p.meta, p.sql)
	fmt.Printf("\n== inspect (%d)\n", len(statements))
	for _, stmt := range statements {
		printInspectStatement(stmt, "  ")
	}
}

func printInspectStatement(stmt core.InspectStatement, indent string) {
	tables := make([]string, 0, len(stmt.Tables))
	for _, t := range stmt.Tables {
		tables = append(tables, qualify(t.Schema, t.Name))
	}
	fmt.Printf("%s%s tables=[%s] fields=[%s] where=[%s]\n",
		indent, stmt.Operation, strings.Join(tables, ", "),
		strings.Join(fieldNames(stmt.Fields), ", "),
		strings.Join(fieldNames(stmt.Where), ", "))
	for _, sub := range stmt.Subqueries {
		printInspectStatement(sub, indent+"  ")
	}
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

// printAnalyzerView shows what the analyzer answered, which separates a sqlglot
// parsing bug from a Go bug in the layer consuming it. It goes through the same
// core/references calls the dialects use, so the probe cannot drift from the
// requests the real completion path sends.
func (p probe) printAnalyzerView() {
	var opts []any
	if p.hasCaret() {
		opts = append(opts, &coreRefs.CompletionCaret{Line: p.caretLine, Col: p.caretCol})
	}

	fmt.Println("\n== analyzer")

	refs, virtual, err := coreRefs.CollectReferencesFromPython(p.analyzer, p.sql, p.dialect, p.meta, opts...)
	if err != nil {
		fmt.Printf("  references: FAILED: %v\n", err)
	} else {
		for _, r := range refs {
			fmt.Printf("  relation   %s alias=%q virtual=%t level=%d at %d:%d\n",
				qualify(r.Schema, r.Table), r.Alias, r.IsVirtual, r.NestingLevel, r.Line, r.Col)
		}
		for _, v := range virtual {
			fmt.Printf("  cte        %s columns=%d\n", v.Table, len(v.Columns))
		}
	}

	columnRefs, aliases, err := coreRefs.CollectColumnRefsFromPython(p.analyzer, p.sql, p.dialect, p.meta, opts...)
	if err != nil {
		fmt.Printf("  column refs: FAILED: %v\n", err)
	} else {
		for _, c := range columnRefs {
			fmt.Printf("  column ref %s qualified=%t resolved=%t level=%d at %d:%d\n",
				c.Column, c.Qualified, c.Resolved, c.NestingLevel, c.Line, c.Col)
		}
		for _, a := range aliases {
			fmt.Printf("  alias      %s\n", a.Alias)
		}
	}

	if !p.hasCaret() {
		return
	}
	completionCtx, err := coreRefs.ParseCompletionContextFromPython(p.analyzer, p.sql, p.caretLine, p.caretCol, p.meta)
	if err != nil {
		fmt.Printf("  context: FAILED: %v\n", err)
		return
	}
	precedingColumn := "none"
	if completionCtx.PrecedingColumn != nil {
		precedingColumn = completionCtx.PrecedingColumn.Name
	}
	fmt.Printf("  context    targets=%d parts=%v afterDot=%t table=%q preceding=%s valuePos=%t\n",
		completionCtx.Targets, completionCtx.Parts, completionCtx.CaretAfterDot,
		completionCtx.TargetTable, precedingColumn, completionCtx.ValuePosition)
}
