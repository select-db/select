package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// normalizeFolder returns an absolute, cleaned path, refusing the two folders
// nobody opens on purpose: a home directory and a filesystem root both mean
// indexing everything the user owns. Anything narrower would get in the way of
// people who keep SQL in odd places.
func normalizeFolder(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no folder given")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s no longer exists", abs)
		}
		return "", fmt.Errorf("read %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", abs)
	}

	// A path is its own parent only at the top of the tree, "/" and "C:\" alike.
	if filepath.Dir(abs) == abs {
		return "", fmt.Errorf("a filesystem root is not a workspace, pick a folder inside it")
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if filepath.Clean(home) == abs {
			return "", fmt.Errorf("your home folder is not a workspace, pick a folder inside it")
		}
	}

	return abs, nil
}
