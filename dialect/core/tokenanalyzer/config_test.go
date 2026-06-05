package tokenanalyzer_test

import (
	"testing"

	"github.com/selectDb/dialect/core/tokenanalyzer"
)

// ---------------------------------------------------------------------------
// matchesGlob (via ResolveLintConfig behaviour)
// ---------------------------------------------------------------------------

func TestMatchesGlob_RelativePath(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "migrations/001.sql")
	if cfg.Rules["S001"].Severity != "off" {
		t.Error("relative path should match migrations/**")
	}
}

func TestMatchesGlob_AbsolutePath(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/project/migrations/001.sql")
	if cfg.Rules["S001"].Severity != "off" {
		t.Error("absolute path should match migrations/**")
	}
}

func TestMatchesGlob_NoMatch(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/seeds/001.sql")
	if _, ok := cfg.Rules["S001"]; ok {
		t.Error("seeds/ should not match migrations/**")
	}
}

func TestMatchesGlob_BasenameGlob(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Files: []string{"*.sql"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "hint"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/abs/queries/report.sql")
	if cfg.Rules["S001"].Severity != "hint" {
		t.Error("*.sql should match any .sql file")
	}
}

func TestMatchesGlob_BasenameNoMatch(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Files: []string{"*.sql"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "hint"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/abs/queries/report.json")
	if _, ok := cfg.Rules["S001"]; ok {
		t.Error("*.sql should not match .json file")
	}
}

// ---------------------------------------------------------------------------
// ParseLintConfig
// ---------------------------------------------------------------------------

func TestParseLintConfig_Empty(t *testing.T) {
	entries, err := tokenanalyzer.ParseLintConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestParseLintConfig_Invalid(t *testing.T) {
	_, err := tokenanalyzer.ParseLintConfig("{not valid json}")
	if err == nil {
		t.Error("expected parse error")
	}
}

func TestParseLintConfig_ValidStructure(t *testing.T) {
	raw := `[
		{"rules": {"S001": "off"}},
		{"files": ["migrations/**"], "rules": {"trailing-comma": "warning"}},
		{"ignores": ["generated/**"]},
		{"custom": [{"id": "C001", "severity": "error", "message": "msg", "pattern": "DROP"}]}
	]`
	entries, err := tokenanalyzer.ParseLintConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Rules["S001"].Severity != "off" {
		t.Error("expected S001=off in first entry")
	}
	if len(entries[1].Files) != 1 || entries[1].Files[0] != "migrations/**" {
		t.Error("expected files in second entry")
	}
	if len(entries[2].Ignores) != 1 {
		t.Error("expected ignores in third entry")
	}
	if len(entries[3].Custom) != 1 || entries[3].Custom[0].ID != "C001" {
		t.Error("expected custom rule in fourth entry")
	}
}

func TestParseLintConfig_RuleTupleForm(t *testing.T) {
	raw := `[{"rules": {"S001": ["warning", {}]}}]`
	entries, err := tokenanalyzer.ParseLintConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Rules["S001"].Severity != "warning" {
		t.Error("expected tuple form to parse severity")
	}
}

// ---------------------------------------------------------------------------
// ResolveLintConfig
// ---------------------------------------------------------------------------

func TestResolveLintConfig_GlobalRuleApplies(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/any/file.sql")
	if cfg.Rules["S001"].Severity != "off" {
		t.Error("expected S001=off")
	}
}

func TestResolveLintConfig_FileScopedRuleApplies(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "warning"}}},
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/migrations/001.sql")
	if cfg.Rules["S001"].Severity != "off" {
		t.Errorf("expected S001=off for migration file, got %q", cfg.Rules["S001"].Severity)
	}
}

func TestResolveLintConfig_FileScopedRuleDoesNotApplyToOtherFiles(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "warning"}}},
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/queries/report.sql")
	if cfg.Rules["S001"].Severity != "warning" {
		t.Errorf("expected S001=warning for non-migration file, got %q", cfg.Rules["S001"].Severity)
	}
}

func TestResolveLintConfig_IgnoresFileReturnsEmpty(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "error"}}},
		{Ignores: []string{"generated/**"}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/generated/out.sql")
	if len(cfg.Rules) != 0 {
		t.Errorf("expected empty rules for ignored file, got %v", cfg.Rules)
	}
}

func TestResolveLintConfig_IgnoresDoesNotAffectOtherFiles(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "error"}}},
		{Ignores: []string{"generated/**"}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/queries/report.sql")
	if cfg.Rules["S001"].Severity != "error" {
		t.Error("expected S001=error for non-ignored file")
	}
}

func TestResolveLintConfig_CustomRuleOffViaScopedRules(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Custom: []tokenanalyzer.CustomRule{
			{ID: "C002", Severity: "error", Message: "no drop", Pattern: "(?i)DROP"},
		}},
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"C002": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/migrations/001.sql")
	if cfg.Rules["C002"].Severity != "off" {
		t.Errorf("expected C002=off for migration file, got %q", cfg.Rules["C002"].Severity)
	}
	cfg2 := tokenanalyzer.ResolveLintConfig(entries, "/workspace/queries/report.sql")
	if len(cfg2.Custom) != 1 {
		t.Error("expected C002 custom rule for non-migration file")
	}
}

func TestResolveLintConfig_EmptyFilePath_OnlyGlobalApplies(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "off"}}},
		{Files: []string{"migrations/**"}, Rules: map[string]tokenanalyzer.LintRuleConfig{"S002": {Severity: "off"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "")
	if cfg.Rules["S001"].Severity != "off" {
		t.Error("expected global S001=off")
	}
	if _, ok := cfg.Rules["S002"]; ok {
		t.Error("file-scoped S002 should not apply when filePath is empty")
	}
}

func TestResolveLintConfig_LaterEntriesWin(t *testing.T) {
	entries := tokenanalyzer.LintFile{
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "warning"}}},
		{Rules: map[string]tokenanalyzer.LintRuleConfig{"S001": {Severity: "error"}}},
	}
	cfg := tokenanalyzer.ResolveLintConfig(entries, "/workspace/file.sql")
	if cfg.Rules["S001"].Severity != "error" {
		t.Errorf("expected later entry to win, got %q", cfg.Rules["S001"].Severity)
	}
}
