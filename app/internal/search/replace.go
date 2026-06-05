package search

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Replace performs a search and replace operation
func (s *Search) Replace(params ReplaceParams) (*ReplaceResult, error) {
	// First, get all matches
	searchParams := SearchParams{
		WorkspaceID:    params.WorkspaceID,
		Pattern:        params.Pattern,
		UseRegex:       params.UseRegex,
		CaseSensitive:  params.CaseSensitive,
		WholeWord:      params.WholeWord,
		IncludePattern: params.IncludePattern,
		ExcludePattern: params.ExcludePattern,
	}

	// If FilePath is specified, only search in that file
	if params.FilePath != "" {
		searchParams.IncludePattern = params.FilePath
	}

	searchResult, err := s.Search(searchParams)
	if err != nil {
		return nil, err
	}

	if params.DryRun {
		// Return what would be replaced without actually replacing
		return &ReplaceResult{
			FilesModified:     searchResult.TotalFiles,
			TotalReplacements: searchResult.TotalMatches,
			ModifiedFiles:     extractFilePaths(searchResult.Files),
		}, nil
	}

	// Build replacement regex
	pattern := params.Pattern
	if !params.UseRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if params.WholeWord {
		pattern = `\b` + pattern + `\b`
	}
	if !params.CaseSensitive {
		pattern = "(?i)" + pattern
	}

	replaceRegex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid replacement pattern: %w", err)
	}

	// Perform actual replacement
	workspacePath, err := s.workspaceRootPath(params.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace: %w", err)
	}

	modifiedFiles := []string{}
	totalReplacements := 0

	for _, fileResult := range searchResult.Files {
		// Containment: skip search-result paths that escape the workspace root.
		if !filepath.IsLocal(fileResult.Path) {
			continue
		}
		filePath := filepath.Join(workspacePath, fileResult.Path)

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Perform replacement
		originalContent := string(content)
		newContent := replaceRegex.ReplaceAllString(originalContent, params.Replacement)

		// Count replacements
		replacements := len(fileResult.Matches)

		// Write back if content changed
		if newContent != originalContent && replacements > 0 {
			err = os.WriteFile(filePath, []byte(newContent), 0644) // #nosec G703 -- filePath contained to the workspace via filepath.IsLocal above
			if err != nil {
				continue // Skip files we can't write
			}
			modifiedFiles = append(modifiedFiles, fileResult.Path)
			totalReplacements += replacements
		}
	}

	return &ReplaceResult{
		FilesModified:     len(modifiedFiles),
		TotalReplacements: totalReplacements,
		ModifiedFiles:     modifiedFiles,
	}, nil
}

// extractFilePaths extracts just the file paths from search results
func extractFilePaths(files []SearchFileResult) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}
