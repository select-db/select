// Package testutil provides shared test helpers for dialect tests.
package testutil

import (
	"os"
	"testing"

	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

// requireAnalyzerEnv makes a missing analyzer fail rather than skip. CI sets
// it, because a skip there stops the run testing what it exists to test.
const requireAnalyzerEnv = "SELECT_REQUIRE_ANALYZER"

// NewTestAnalyzer creates a Python analyzer for tests. A fresh checkout has no
// venv until `uv sync` runs, so the default is to skip.
func NewTestAnalyzer(t *testing.T) *ta.Analyzer {
	t.Helper()
	pythonPath, script, ok := ta.FindDevAnalyzer()
	if !ok {
		const missing = "python venv not found; run `uv sync` in dialect/core/tokenanalyzer/python"
		if os.Getenv(requireAnalyzerEnv) != "" {
			t.Fatalf("%s is set: %s", requireAnalyzerEnv, missing)
		}
		t.Skip(missing)
	}
	return ta.NewAnalyzer(pythonPath, script)
}
