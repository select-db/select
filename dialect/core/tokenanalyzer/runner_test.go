package tokenanalyzer_test

import (
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/postgresql"
)

func ruleIDs(diags []tokenanalyzer.Diagnostic) []string {
	ids := make([]string, len(diags))
	for i, d := range diags {
		ids[i] = d.RuleID
	}
	return ids
}

func TestRunner_NoAnalyzer_ReturnsNil(t *testing.T) {
	r := tokenanalyzer.NewLintRunner()
	d := postgresql.NewDialect()
	diags := r.Run("SELECT * FROM t WHERE x = NULL", d, core.Metadata{}, tokenanalyzer.ResolvedLintConfig{})
	if diags != nil {
		t.Errorf("expected nil without analyzer, got %v", diags)
	}
}

func TestRunner_EmptySQL_ReturnsNil(t *testing.T) {
	r := tokenanalyzer.NewLintRunner()
	d := postgresql.NewDialect()
	if r.Run("", d, core.Metadata{}, tokenanalyzer.ResolvedLintConfig{}) != nil {
		t.Error("expected nil for empty SQL")
	}
}
