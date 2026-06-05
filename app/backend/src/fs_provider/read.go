package fs_provider

import (
	"os"
	"strings"
)

type ReadFileParams struct {
	URI       string `json:"uri"`
	StartLine *int   `json:"startLine,omitempty"`
	EndLine   *int   `json:"endLine,omitempty"`
}

func (fsp *FSProvider) ReadFile(params ReadFileParams) (string, error) {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)

	// If line range is specified, extract only those lines
	if params.StartLine != nil || params.EndLine != nil {
		lines := strings.Split(content, "\n")
		start := 0
		end := len(lines)

		if params.StartLine != nil {
			startIdx := *params.StartLine - 1 // Convert to 0-indexed
			if startIdx < 0 {
				startIdx = 0
			}
			if startIdx < len(lines) {
				start = startIdx
			} else {
				// Start beyond file end, return empty
				return "", nil
			}
		}

		if params.EndLine != nil {
			endIdx := *params.EndLine // EndLine is inclusive, so use as-is for slice
			if endIdx < start {
				// Invalid range, return empty
				return "", nil
			}
			if endIdx < len(lines) {
				end = endIdx
			}
			// If endIdx >= len(lines), use full length (already set)
		}

		selectedLines := lines[start:end]
		return strings.Join(selectedLines, "\n"), nil
	}

	return content, nil
}

type ReadDirectoryParams struct {
	URI string `json:"uri"`
}

func (fsp *FSProvider) ReadDirectory(params ReadDirectoryParams) ([]DirEntry, error) {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{
			Name: e.Name(),
			Type: dirEntryType(e),
		})
	}

	return out, nil
}
