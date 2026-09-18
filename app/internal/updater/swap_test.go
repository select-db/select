package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Replacing the running binary is the irreversible step: if it goes wrong the
// user is left with an app that will not start, and no way to update it. The
// invariant worth testing is not the sequence of renames but its consequence —
// at every point, either the old binary or the new one is in place.

func installed(t *testing.T) (dst string) {
	t.Helper()

	dst = filepath.Join(t.TempDir(), "select")
	if err := os.WriteFile(dst, []byte("the old binary"), 0o755); err != nil {
		t.Fatalf("write installed binary: %v", err)
	}
	return dst
}

func downloaded(t *testing.T, content string) string {
	t.Helper()

	src := filepath.Join(t.TempDir(), "select.new")
	if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
		t.Fatalf("write downloaded binary: %v", err)
	}
	return src
}

func contents(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func TestSwapInstallsTheNewBinary(t *testing.T) {
	dst := installed(t)
	src := downloaded(t, "the new binary")

	if err := atomicSwap(src, dst); err != nil {
		t.Fatalf("swap: %v", err)
	}

	if got := contents(t, dst); got != "the new binary" {
		t.Errorf("installed binary is %q", got)
	}
	// The previous one is kept until the next successful start, which is what
	// makes the swap recoverable by hand if the new build will not run.
	if got := contents(t, dst+".bak"); got != "the old binary" {
		t.Errorf("backup is %q", got)
	}
	if _, err := os.Stat(dst + ".new"); err == nil {
		t.Error("the staged copy should be gone once it is in place")
	}
}

// A binary that is not executable is a binary that will not start, and the
// downloaded file does not carry the mode the installed one had.
func TestSwapKeepsTheBinaryExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no executable bit on Windows")
	}

	dst := installed(t)
	src := downloaded(t, "the new binary") // written 0o600

	if err := atomicSwap(src, dst); err != nil {
		t.Fatalf("swap: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed binary is not executable: %v", info.Mode())
	}
}

// The failure that matters: the update cannot land, and the user still has a
// working app.
func TestFailedSwapLeavesTheOldBinaryInPlace(t *testing.T) {
	dst := installed(t)
	missing := filepath.Join(t.TempDir(), "was-never-downloaded")

	if err := atomicSwap(missing, dst); err == nil {
		t.Fatal("swapping in a file that does not exist should fail")
	}

	if got := contents(t, dst); got != "the old binary" {
		t.Errorf("installed binary is %q, should be untouched", got)
	}
	if _, err := os.Stat(dst + ".new"); err == nil {
		t.Error("a half-staged copy should not be left behind")
	}
}

// A previous attempt that died mid-swap leaves a stale staged copy; the next
// one has to overwrite it rather than trip over it.
func TestSwapOverwritesAStaleStagedCopy(t *testing.T) {
	dst := installed(t)
	src := downloaded(t, "the new binary")

	if err := os.WriteFile(dst+".new", []byte("left over from last time"), 0o600); err != nil {
		t.Fatalf("write stale copy: %v", err)
	}

	if err := atomicSwap(src, dst); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if got := contents(t, dst); got != "the new binary" {
		t.Errorf("installed binary is %q", got)
	}
}
