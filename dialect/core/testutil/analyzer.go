// Package testutil provides shared test helpers for dialect tests.
package testutil

import (
	"testing"

	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

// NewTestAnalyzer creates a Python analyzer for tests. Skips if the venv is not found.
func NewTestAnalyzer(t *testing.T) *ta.Analyzer {
	t.Helper()
	pythonPath, script, ok := ta.FindDevAnalyzer()
	if !ok {
		t.Skip("python venv not found; run `uv sync` in dialect/core/tokenanalyzer/python")
	}
	return ta.NewAnalyzer(pythonPath, script)
}
