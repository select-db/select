package tokenanalyzer

import (
	"strings"

	"github.com/selectDb/dialect/core"
)

const maxDiagnostics = 50

// LintRunner delegates all lint checks to the pysqlglot Analyzer subprocess.
type LintRunner struct {
	Analyzer *Analyzer
}

func NewLintRunner() *LintRunner {
	return &LintRunner{}
}

// Run delegates to the Analyzer, appends custom rule matches, then applies
// severity overrides from cfg, and filters any diagnostics suppressed by
// inline disable comments. Returns nil when no analyzer is configured or
// when sql is empty. The result is capped at maxDiagnostics.
func (r *LintRunner) Run(sql string, d core.SQLDialect, meta core.Metadata, cfg ResolvedLintConfig) []Diagnostic {
	if sql == "" || d == nil || r.Analyzer == nil {
		return nil
	}
	raw := r.Analyzer.Analyze(sql, d, meta)
	raw = append(raw, ApplyCustomRules(sql, cfg.Custom)...)

	diags := make([]Diagnostic, 0, len(raw))
	for _, diag := range raw {
		if override, ok := cfg.Rules[diag.RuleID]; ok {
			if override.Severity == "off" {
				continue
			}
			diag.Severity = parseCustomSeverity(override.Severity)
		}
		diags = append(diags, diag)
	}

	diags = filterDisabled(sql, diags)

	if len(diags) > maxDiagnostics {
		return diags[:maxDiagnostics]
	}
	return diags
}

// filterDisabled removes diagnostics suppressed by inline disable comments:
//
//   - "-- lint-disable <rule-id>" anywhere in the file suppresses that rule globally.
//   - "-- lint-disable-next-line <rule-id>" suppresses that rule on the immediately
//     following line.
func filterDisabled(sql string, diags []Diagnostic) []Diagnostic {
	if len(diags) == 0 {
		return diags
	}

	lines := strings.Split(sql, "\n")

	// file-level disabled rules
	fileOff := map[string]bool{}
	// per-line disabled rules: nextLineOff[lineNum] = set of rule IDs (1-based line)
	nextLineOff := map[int]map[string]bool{}

	for i, line := range lines {
		lineNum := i + 1 // 1-based
		lower := strings.ToLower(line)

		if idx := strings.Index(lower, "-- lint-disable-next-line "); idx != -1 {
			id := strings.TrimSpace(line[idx+len("-- lint-disable-next-line "):])
			if id != "" {
				next := lineNum + 1
				if nextLineOff[next] == nil {
					nextLineOff[next] = map[string]bool{}
				}
				nextLineOff[next][id] = true
			}
			continue
		}
		if idx := strings.Index(lower, "-- lint-disable "); idx != -1 {
			id := strings.TrimSpace(line[idx+len("-- lint-disable "):])
			if id != "" {
				fileOff[id] = true
			}
		}
	}

	if len(fileOff) == 0 && len(nextLineOff) == 0 {
		return diags
	}

	out := diags[:0:0]
	for _, d := range diags {
		if fileOff[d.RuleID] {
			continue
		}
		if nextLineOff[d.StartLine][d.RuleID] {
			continue
		}
		out = append(out, d)
	}
	return out
}
