package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// install unzips the Windows archive, swaps select.exe and the sibling
// analyzer.exe, and relaunches.
//
// On NTFS the running .exe can be renamed even while the process is live: the
// open handle keeps pointing at the file by inode, and the directory entry is
// free for the replacement. atomicSwap renames the running file to .bak first,
// then moves the new binary into its place.
//
// Child processes outlive the parent on Windows by default, so no detach flag
// is required for the relaunch.
func install(archivePath, tmpDir string) error {
	if err := unzip(archivePath, tmpDir); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	newBin := filepath.Join(tmpDir, "select.exe")
	if _, err := os.Stat(newBin); err != nil {
		return fmt.Errorf("select.exe not found after unzip")
	}

	current, err := os.Executable()
	if err != nil {
		return err
	}
	if err := atomicSwap(newBin, current); err != nil {
		return err
	}

	// Sibling analyzer.exe ships in the same archive; swap if present.
	if newAnalyzer := filepath.Join(tmpDir, "analyzer.exe"); fileExists(newAnalyzer) {
		currentAnalyzer := filepath.Join(filepath.Dir(current), "analyzer.exe")
		_ = atomicSwap(newAnalyzer, currentAnalyzer)
	}

	return exec.Command(current).Start()
}
