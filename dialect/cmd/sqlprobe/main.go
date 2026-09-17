// Command sqlprobe runs one SQL string, against one hand-written catalog,
// through every dialect-level language feature at once and prints what each
// produced. It is the isolation step for a suspected completion or lint bug:
// shrink the case here until exactly one layer is wrong, then write the
// regression test against that layer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/engine"
)

func main() {
	dialectName := flag.String("dialect", "postgresql", "postgresql, mysql or sqlite")
	schemaDSL := flag.String("schema", "", "catalog in probe notation, or @file")
	sqlInput := flag.String("sql", "", "SQL to probe, or @file. A | marks the caret")
	defaultSchema := flag.String("default-schema", "", "default schema (per-dialect default when empty)")
	metaJSON := flag.String("meta", "", "core.Metadata JSON file, instead of -schema")
	showRaw := flag.Bool("raw", false, "also dump the raw analyzer responses")
	flag.Parse()

	if err := run(*dialectName, *schemaDSL, *sqlInput, *defaultSchema, *metaJSON, *showRaw); err != nil {
		fmt.Fprintf(os.Stderr, "sqlprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(dialectName, schemaDSL, sqlInput, defaultSchema, metaJSON string, showRaw bool) error {
	d := engine.GetDialect(dialectName)
	if d == nil {
		return fmt.Errorf("unknown dialect %q (want postgresql, mysql or sqlite)", dialectName)
	}
	if defaultSchema == "" {
		defaultSchema = d.DefaultSchemaName()
		if defaultSchema == "" {
			defaultSchema = "def" // MySQL has no schemas; give the catalog one bucket
		}
	}

	sqlText, err := readArg(sqlInput)
	if err != nil {
		return fmt.Errorf("reading -sql: %w", err)
	}
	if strings.TrimSpace(sqlText) == "" {
		return fmt.Errorf("-sql is required")
	}

	meta, err := loadMetadata(schemaDSL, metaJSON, defaultSchema)
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

	fmt.Printf("== dialect %s   default schema %s\n", d.Name(), meta.DefaultSchema)
	printSQL(sqlText, caretLine, caretCol)
	printCatalog(meta)
	printLint(analyzer, d, meta, sqlText)
	if caretLine > 0 {
		printCompletion(d, meta, sqlText, caretLine, caretCol)
	}
	printInspect(d, meta, sqlText)
	if showRaw {
		printRaw(analyzer, d, meta, sqlText, caretLine, caretCol)
	}
	return nil
}

func loadMetadata(schemaDSL, metaJSON, defaultSchema string) (core.Metadata, error) {
	if metaJSON != "" {
		raw, err := os.ReadFile(metaJSON)
		if err != nil {
			return core.Metadata{}, fmt.Errorf("reading -meta: %w", err)
		}
		var meta core.Metadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return core.Metadata{}, fmt.Errorf("parsing -meta: %w", err)
		}
		return meta, nil
	}

	dsl, err := readArg(schemaDSL)
	if err != nil {
		return core.Metadata{}, fmt.Errorf("reading -schema: %w", err)
	}
	return parseSchemaDSL(dsl, defaultSchema)
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

func printSQL(sql string, caretLine, caretCol int) {
	fmt.Println("\n== sql")
	for i, line := range strings.Split(sql, "\n") {
		fmt.Printf("  %2d | %s\n", i+1, line)
		if i+1 == caretLine {
			fmt.Printf("     | %s^ caret %d:%d\n", strings.Repeat(" ", caretCol), caretLine, caretCol)
		}
	}
}

func printCatalog(meta core.Metadata) {
	fmt.Println("\n== catalog")
	for _, s := range meta.Schemas {
		relations := len(s.Tables) + len(s.Views) + len(s.MaterializedViews)
		if relations == 0 && len(s.Functions) == 0 {
			continue
		}
		fmt.Printf("  schema %s: %d relation(s), %d function(s)\n", s.Name, relations, len(s.Functions))
		for _, t := range allRelations(s) {
			var cols []string
			for _, c := range t.Columns {
				label := c.Name + ":" + c.Type
				if len(c.EnumValues) > 0 {
					label += "{" + strings.Join(c.EnumValues, "|") + "}"
				}
				cols = append(cols, label)
			}
			fmt.Printf("    %s(%s)\n", t.Name, strings.Join(cols, ", "))
		}
	}
}

func allRelations(s core.Schema) []core.Table {
	all := append([]core.Table{}, s.Tables...)
	all = append(all, s.Views...)
	return append(all, s.MaterializedViews...)
}

func printLint(analyzer *tokenanalyzer.Analyzer, d core.SQLDialect, meta core.Metadata, sql string) {
	runner := tokenanalyzer.NewLintRunner()
	runner.Analyzer = analyzer
	diagnostics := runner.Run(sql, d, meta, tokenanalyzer.ResolvedLintConfig{
		Rules: map[string]tokenanalyzer.LintRuleConfig{},
	})

	fmt.Printf("\n== lint (%d)\n", len(diagnostics))
	for _, diag := range diagnostics {
		fmt.Printf("  %-7s %-28s %d:%d-%d:%d  %s\n",
			severityName(diag.Severity), diag.RuleID,
			diag.StartLine, diag.StartCol, diag.EndLine, diag.EndCol,
			diag.Message)
	}
}

func severityName(s tokenanalyzer.Severity) string {
	switch s {
	case tokenanalyzer.SeverityError:
		return "error"
	case tokenanalyzer.SeverityWarning:
		return "warning"
	default:
		return "hint"
	}
}

func printCompletion(d core.SQLDialect, meta core.Metadata, sql string, caretLine, caretCol int) {
	candidates, err := d.Complete(context.Background(), sql, caretLine, caretCol, meta)
	if err != nil {
		fmt.Printf("\n== complete: FAILED: %v\n", err)
		return
	}

	byType := map[string][]string{}
	var order []string
	for _, c := range candidates {
		name := candidateTypeName(c.Type)
		if _, seen := byType[name]; !seen {
			order = append(order, name)
		}
		label := c.Text
		if c.InsertText != "" && c.InsertText != c.Text {
			label += " -> " + c.InsertText
		}
		byType[name] = append(byType[name], label)
	}

	fmt.Printf("\n== complete @ %d:%d (%d)\n", caretLine, caretCol, len(candidates))
	for _, name := range order {
		fmt.Printf("  %-9s %s\n", name, strings.Join(byType[name], ", "))
	}
}

func candidateTypeName(t core.CandidateType) string {
	switch t {
	case core.CandidateTypeKeyword:
		return "keyword"
	case core.CandidateTypeSchema:
		return "schema"
	case core.CandidateTypeTable:
		return "table"
	case core.CandidateTypeForeignTable:
		return "fgntable"
	case core.CandidateTypeView:
		return "view"
	case core.CandidateTypeMaterializedView:
		return "matview"
	case core.CandidateTypeColumn:
		return "column"
	case core.CandidateTypeOperator:
		return "operator"
	case core.CandidateTypeType:
		return "type"
	case core.CandidateTypeFunction:
		return "function"
	case core.CandidateTypeEnumValue:
		return "enumval"
	case core.CandidateTypeSetting:
		return "setting"
	default:
		return fmt.Sprintf("type%d", int(t))
	}
}

func printInspect(d core.SQLDialect, meta core.Metadata, sql string) {
	statements := d.Inspect(meta, sql)
	fmt.Printf("\n== inspect (%d)\n", len(statements))
	for _, stmt := range statements {
		printInspectStatement(stmt, "  ")
	}
}

func printInspectStatement(stmt core.InspectStatement, indent string) {
	var tables []string
	for _, t := range stmt.Tables {
		tables = append(tables, qualify(t.Schema, t.Name))
	}
	fmt.Printf("%s%v tables=[%s] fields=[%s] where=[%s]\n",
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

// printRaw shows the analyzer's own answers, which separates a Python parsing
// bug from a Go bug in the layer that consumes it. The requests mirror the ones
// core/references sends, field for field, so what the probe prints is what the
// real completion path saw.
func printRaw(analyzer *tokenanalyzer.Analyzer, d core.SQLDialect, meta core.Metadata, sql string, caretLine, caretCol int) {
	schemaDict := core.MetaToSchemaDict(meta)
	defaultSchema := core.GetDefaultSchema(meta)

	requests := []map[string]any{
		{"action": "collect_references", "sql": sql, "dialect": d.Name(), "schema": schemaDict, "default_schema": defaultSchema},
		{"action": "collect_column_refs", "sql": sql, "dialect": d.Name(), "schema": schemaDict, "default_schema": defaultSchema},
		{"action": "lint", "sql": sql, "dialect": d.Name(), "schema": schemaDict, "default_schema": defaultSchema},
	}
	if caretLine > 0 {
		for _, req := range requests[:2] {
			req["caret_line"] = caretLine
			req["caret_col"] = caretCol
		}
		requests = append(requests, map[string]any{
			"action": "complete_context", "sql": sql,
			"caret_line": caretLine, "caret_col": caretCol, "schema": schemaDict,
		})
	}

	fmt.Println("\n== raw analyzer")
	for _, req := range requests {
		resp, err := analyzer.Call(req)
		if err != nil {
			fmt.Printf("  %s: FAILED: %v\n", req["action"], err)
			continue
		}
		fmt.Printf("  %s: %s\n", req["action"], string(resp))
	}
}
