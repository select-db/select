package search

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Embed ripgrep binaries for different platforms
// These will be compiled into the Go binary
//
//go:embed bin/rg-darwin-x64
var rgDarwinX64 []byte

//go:embed bin/rg-darwin-arm64
var rgDarwinARM64 []byte

//go:embed bin/rg-linux-x64
var rgLinuxX64 []byte

//go:embed bin/rg-windows-x64.exe
var rgWindowsX64 []byte

var (
	extractedBinaryPath string
	extractOnce         sync.Once
	extractErr          error
)

// getRipgrepBinary returns the path to the ripgrep binary.
// On first call, it extracts the embedded binary to a temp directory.
// Subsequent calls return the cached path.
func getRipgrepBinary() (string, error) {
	extractOnce.Do(func() {
		extractedBinaryPath, extractErr = extractRipgrepBinary()
	})
	return extractedBinaryPath, extractErr
}

// extractRipgrepBinary extracts the appropriate ripgrep binary for the current platform
// to a temporary directory and returns the path to it.
func extractRipgrepBinary() (string, error) {
	// Determine which binary to use based on platform
	var binaryData []byte
	var binaryName string

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			binaryData = rgDarwinARM64
			binaryName = "rg-darwin-arm64"
		} else {
			binaryData = rgDarwinX64
			binaryName = "rg-darwin-x64"
		}
	case "linux":
		binaryData = rgLinuxX64
		binaryName = "rg-linux-x64"
	case "windows":
		binaryData = rgWindowsX64
		binaryName = "rg-windows-x64.exe"
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Create temp directory for the binary
	// Use a consistent directory name so we don't extract multiple copies
	tempDir := filepath.Join(os.TempDir(), "selectdb-ripgrep")
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write the binary to the temp directory
	binaryPath := filepath.Join(tempDir, binaryName)

	// Check if binary already exists and is valid
	if fileInfo, err := os.Stat(binaryPath); err == nil {
		// Binary exists, check if it's the same size (simple validation)
		if int(fileInfo.Size()) == len(binaryData) {
			// Binary already extracted, just return the path
			return binaryPath, nil
		}
	}

	// Write the binary
	err = os.WriteFile(binaryPath, binaryData, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to write binary: %w", err)
	}

	// Make sure it's executable
	err = os.Chmod(binaryPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	return binaryPath, nil
}
