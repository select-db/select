package tokenanalyzer

import (
	"regexp"
)

// CustomRule is a user-defined regexp-based lint rule.
type CustomRule struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // "off", "hint", "warning", "error"
	Message  string `json:"message"`
	Pattern  string `json:"pattern"`
}

// ApplyCustomRules runs each custom rule against sql and returns matching diagnostics.
// Patterns are matched against the full SQL string so they can span multiple lines.
func ApplyCustomRules(sql string, rules []CustomRule) []Diagnostic {
	var diags []Diagnostic
	for _, rule := range rules {
		if rule.Pattern == "" || rule.ID == "" {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		severity := parseCustomSeverity(rule.Severity)
		for _, loc := range re.FindAllStringIndex(sql, -1) {
			startLine, startCol := offsetToLineCol(sql, loc[0])
			endLine, endCol := offsetToLineCol(sql, loc[1])
			diags = append(diags, Diagnostic{
				RuleID:    rule.ID,
				Severity:  severity,
				Message:   rule.Message,
				StartLine: startLine,
				StartCol:  startCol,
				EndLine:   endLine,
				EndCol:    endCol,
			})
		}
	}
	return diags
}

// offsetToLineCol converts a byte offset in s to 1-based line and 0-based column.
func offsetToLineCol(s string, offset int) (line, col int) {
	line = 1
	for i, ch := range s {
		if i == offset {
			return line, col
		}
		if ch == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func parseCustomSeverity(s string) Severity {
	switch s {
	case "error":
		return SeverityError
	case "hint":
		return SeverityHint
	default:
		return SeverityWarning
	}
}
