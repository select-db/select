package updater

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
)

// install unzips the macOS archive, swaps the .app bundle into /Applications,
// and relaunches detached from the parent process group.
func install(archivePath, tmpDir string) error {
	if err := unzip(archivePath, tmpDir); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, "*.app"))
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("no .app bundle found after unzip")
	}
	newApp := matches[0]

	// Always install into /Applications, avoids Desktop/Downloads TCC permission prompts.
	currentApp := filepath.Join("/Applications", filepath.Base(newApp))

	if err := atomicSwap(newApp, currentApp); err != nil {
		return err
	}

	// Detach from parent process group so the relaunched app survives wailsruntime.Quit().
	cmd := exec.Command("open", "-n", currentApp)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}
