// Package testutil provides shared test helpers for dialect tests.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

// NewTestAnalyzer creates a Python analyzer for tests. Skips if the venv is not found.
func NewTestAnalyzer(t *testing.T) *ta.Analyzer {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	pyDir := filepath.Join(filepath.Dir(thisFile), "..", "tokenanalyzer", "python")
	script := filepath.Join(pyDir, "main.py")
	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	pyPath := filepath.Join(pyDir, ".venv", venvBin, "python3")
	if _, err := os.Stat(pyPath); err != nil {
		t.Skipf("python venv not found at %s", pyPath)
	}
	return ta.NewAnalyzer(pyPath, script)
}
