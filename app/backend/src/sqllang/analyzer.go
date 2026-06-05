package sqllang

import (
	"os"
	"path/filepath"
	"runtime"
)

// resolveAnalyzer finds the analyzer executable and optional arguments.
// In dev, it looks for the venv python + main.py in the source tree.
// Otherwise, it looks for the binary adjacent to the executable.
func resolveAnalyzer() (execPath string, args []string, ok bool) {
	if os.Getenv("APP_ENV") == "dev" {
		return resolveDevAnalyzer()
	}

	exe, err := os.Executable()
	if err != nil {
		return "", nil, false
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}

	name := "analyzer"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	candidate := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil, true
	}

	return "", nil, false
}

// resolveDevAnalyzer looks for a venv python + main.py under dialect/core/lint/python
// in parent directories of the working directory and the binary.
func resolveDevAnalyzer() (execPath string, args []string, ok bool) {
	roots := searchRoots()

	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}

	for _, root := range roots {
		pyDir := filepath.Join(root, "dialect", "core", "tokenanalyzer", "python")
		script := filepath.Join(pyDir, "main.py")
		if _, err := os.Stat(script); err != nil {
			continue
		}

		for _, py := range []string{"python3", "python"} {
			pyPath := filepath.Join(pyDir, ".venv", venvBin, py)
			if _, err := os.Stat(pyPath); err == nil {
				return pyPath, []string{script}, true
			}
		}
	}
	return "", nil, false
}

// searchRoots returns the repo roots discovered from cwd and the binary directory.
func searchRoots() []string {
	var roots []string
	if cwd, err := os.Getwd(); err == nil {
		if root, found := findRepoRoot(cwd); found {
			roots = append(roots, root)
		}
	}
	if exe, err := os.Executable(); err == nil {
		if root, found := findRepoRoot(filepath.Dir(exe)); found {
			roots = append(roots, root)
		}
	}
	return roots
}

// findRepoRoot walks up from start until it finds a go.work or .git entry.
func findRepoRoot(start string) (string, bool) {
	dir := start
	for {
		for _, marker := range []string{"go.work", ".git"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
