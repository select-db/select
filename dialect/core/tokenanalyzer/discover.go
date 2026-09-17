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

	pyDir := filepath.Join(filepath.Dir(thisFile), "python")
	script = filepath.Join(pyDir, "main.py")
	if _, err := os.Stat(script); err != nil {
		return "", "", false
	}

	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	for _, name := range []string{"python3", "python"} {
		candidate := filepath.Join(pyDir, ".venv", venvBin, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, script, true
		}
	}
	return "", "", false
}
