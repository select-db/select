package main

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

func printResult(result Result, meta core.Metadata) {
	fmt.Printf("== dialect %s   default schema %s\n", result.Dialect, core.GetDefaultSchema(meta))

	sqlText, _, _ := cutCaret(result.SQL)
	fmt.Println("\n== sql")
	for i, line := range strings.Split(sqlText, "\n") {
		fmt.Printf("  %2d | %s\n", i+1, line)
		if result.Caret != nil && i+1 == result.Caret.Line {
			fmt.Printf("     | %s^ caret %d:%d\n", strings.Repeat(" ", result.Caret.Col), result.Caret.Line, result.Caret.Col)
		}
	}

	fmt.Printf("\n== lint (%d)\n", len(result.Lint))
	for _, diag := range result.Lint {
		fmt.Printf("  %-7s %-28s %d:%d-%d:%d  %s\n",
			diag.Severity, diag.Rule, diag.StartLine, diag.StartCol, diag.EndLine, diag.EndCol, diag.Message)
	}

	if result.Caret != nil {
		printCompletion(result)
	}

	fmt.Printf("\n== inspect (%d)\n", len(result.Inspect))
	for _, stmt := range result.Inspect {
		printStatement(stmt, "  ", "")
	}

	fmt.Printf("\n== permissions  needs=[%s] converged=%t\n",
		strings.Join(result.Permissions.Needs, ", "), result.Permissions.Converged)
	for _, verdict := range result.Permissions.Policies {
		outcome := "ALLOWED"
		if !verdict.Allowed {
			outcome = "denied: " + verdict.Denial
		}
		fmt.Printf("  %-9s %s\n", verdict.Policy, outcome)
	}

	if len(result.Analyzer) > 0 {
		fmt.Println("\n== analyzer")
		for _, line := range result.Analyzer {
			fmt.Printf("  %s\n", line)
		}
	}
}

func printCompletion(result Result) {
	if result.CompletionError != "" {
		fmt.Printf("\n== complete: FAILED: %s\n", result.CompletionError)
		return
	}
	var order []string
	byType := map[string][]string{}
	for _, s := range result.Completion {
		label := s.Text
		if s.Insert != "" {
			label += " -> " + s.Insert
		}
		if _, seen := byType[s.Type]; !seen {
			order = append(order, s.Type)
		}
		byType[s.Type] = append(byType[s.Type], label)
	}
	fmt.Printf("\n== complete @ %d:%d (%d)\n", result.Caret.Line, result.Caret.Col, result.CompletionTotal)
	for _, candidateType := range order {
		fmt.Printf("  %-17s %s\n", candidateType, strings.Join(byType[candidateType], ", "))
	}
}

func printStatement(stmt Statement, indent, role string) {
	marker := role
	if stmt.Filter {
		marker += "filter "
	}
	fmt.Printf("%s%s%s tables=[%s] fields=[%s] where=[%s]\n",
		indent, marker, stmt.Op, strings.Join(stmt.Tables, ", "),
		strings.Join(stmt.Fields, ", "), strings.Join(stmt.Where, ", "))
	for _, sub := range stmt.Subqueries {
		printStatement(sub, indent+"  ", "")
	}
	for _, also := range stmt.Also {
		printStatement(also, indent+"  ", "also ")
	}
}
