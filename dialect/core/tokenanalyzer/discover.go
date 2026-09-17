package tokenanalyzer

import (
	"os"
	"path/filepath"
	"runtime"
)

// FindDevAnalyzer locates the development venv interpreter and main.py, both
// resolved relative to this source file so the caller's working directory does
// not matter. Returns ok=false when the venv has not been created yet, which is
// the normal state of a fresh checkout until `uv sync` runs.
func FindDevAnalyzer() (pythonPath, script string, ok bool) {
	_, thisFile, _, callerOK := runtime.Caller(0)
	if !callerOK {
		return "", "", false
	}

	pythonDir := filepath.Join(filepath.Dir(thisFile), "python")
	script = filepath.Join(pythonDir, "main.py")
	if _, err := os.Stat(script); err != nil {
		return "", "", false
	}

	venvBinDir := "bin"
	if runtime.GOOS == "windows" {
		venvBinDir = "Scripts"
	}
	for _, interpreterName := range []string{"python3", "python"} {
		interpreterPath := filepath.Join(pythonDir, ".venv", venvBinDir, interpreterName)
		if _, err := os.Stat(interpreterPath); err == nil {
			return interpreterPath, script, true
		}
	}
	return "", "", false
}
