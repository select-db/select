package tokenanalyzer_test

import (
	"slices"
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
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

// A builtin the catalog does not list is still no unknown function, and a
// misspelt one still is: the rule must tell the two apart, not go quiet.
func TestRunner_UnknownFunctionConsultsBuiltins(t *testing.T) {
	an := testutil.NewTestAnalyzer(t)
	defer an.Close()
	r := &tokenanalyzer.LintRunner{Analyzer: an}
	meta := core.GetInspectTestMetadata()

	tests := []struct {
		dialect core.SQLDialect
		sql     string
		unknown bool
	}{
		{mysql.NewDialect(), "SELECT JSON_OVERLAPS(c2, c2) FROM main.t1", false},
		{mysql.NewDialect(), "SELECT UUID_TO_BIN(c2) FROM main.t1", false},
		{mysql.NewDialect(), "SELECT JSON_OVERLAPZ(c2, c2) FROM main.t1", true},
		{postgresql.NewDialect(), "SELECT jsonb_path_exists(c2::jsonb, '$.a') FROM main.t1", false},
		{postgresql.NewDialect(), "SELECT pg_typeof(c1) FROM main.t1", false},
		{postgresql.NewDialect(), "SELECT pg_typeoff(c1) FROM main.t1", true},
		{sqlite.NewDialect(), "SELECT json_extract(c2, '$.a') FROM main.t1", false},
		{sqlite.NewDialect(), "SELECT json_extractt(c2, '$.a') FROM main.t1", true},
	}
	for _, tt := range tests {
		t.Run(tt.dialect.Name()+"/"+tt.sql, func(t *testing.T) {
			diags := r.Run(tt.sql, tt.dialect, meta, tokenanalyzer.ResolvedLintConfig{})
			if got := slices.Contains(ruleIDs(diags), "unknown-function"); got != tt.unknown {
				t.Errorf("unknown-function reported: %v, want %v; diagnostics %+v", got, tt.unknown, diags)
			}
		})
	}
}
