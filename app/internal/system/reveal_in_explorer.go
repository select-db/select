package system

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RevealInExplorer opens the system file explorer and highlights the file at
// the given selectdb:// URI.
func (s *System) RevealInExplorer(uri string) error {
	absPath, err := s.FSProvider.GetOSPathFromURI(uri)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", absPath)
	case "darwin":
		cmd = exec.Command("open", "-R", absPath)
	case "linux":
		// On Linux, most file managers don't support selecting a file,
		// so we open the parent directory instead
		cmd = exec.Command("xdg-open", filepath.Dir(absPath))
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to reveal in explorer: %w", err)
	}

	return nil
}
