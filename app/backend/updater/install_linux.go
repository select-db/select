package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// install unzips the Linux archive, swaps the running binary and sibling
// analyzer, and relaunches detached from the parent process group.
//
// On Linux, renaming the currently-running ELF is safe: the kernel keeps the
// existing mapping valid via the open inode.
func install(archivePath, tmpDir string) error {
	if err := unzip(archivePath, tmpDir); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	// Wails outputs `select` (wails.json outputfilename), packaged into the
	// archive under the same name by .github/workflows/release.yml.
	newBin := filepath.Join(tmpDir, "select")
	if _, err := os.Stat(newBin); err != nil {
		return fmt.Errorf("select binary not found after unzip")
	}
	if err := os.Chmod(newBin, 0o755); err != nil {
		return err
	}

	current, err := os.Executable()
	if err != nil {
		return err
	}
	if err := atomicSwap(newBin, current); err != nil {
		return err
	}

	// Sibling analyzer ships in the same archive; swap if present. Best effort:
	// the old analyzer keeps working if the new one fails to land.
	if newAnalyzer := filepath.Join(tmpDir, "analyzer"); fileExists(newAnalyzer) {
		_ = os.Chmod(newAnalyzer, 0o755)
		currentAnalyzer := filepath.Join(filepath.Dir(current), "analyzer")
		_ = atomicSwap(newAnalyzer, currentAnalyzer)
	}

	cmd := exec.Command(current)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}
