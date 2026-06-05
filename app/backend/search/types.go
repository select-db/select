package search

import "selectDb/backend/graph"

// SearchParams contains parameters for searching files
type SearchParams struct {
	WorkspaceID    string `json:"workspaceId"`
	Pattern        string `json:"pattern"`
	UseRegex       bool   `json:"useRegex"`
	CaseSensitive  bool   `json:"caseSensitive"`
	WholeWord      bool   `json:"wholeWord"`
	IncludePattern string `json:"includePattern"` // File pattern to include (e.g., "*.go")
	ExcludePattern string `json:"excludePattern"` // File pattern to exclude
}

// SearchMatch represents a single match within a file
type SearchMatch struct {
	Line       int    `json:"line"`       // Line number (1-indexed)
	Column     int    `json:"column"`     // Column number (1-indexed)
	LineText   string `json:"lineText"`   // The full line content
	MatchStart int    `json:"matchStart"` // Start position of match within line
	MatchEnd   int    `json:"matchEnd"`   // End position of match within line
}

// SearchFileResult represents all matches in a single file
type SearchFileResult struct {
	Path    string        `json:"path"`    // Relative path from workspace root
	Matches []SearchMatch `json:"matches"` // All matches in this file
}

// SearchResult represents the complete search results
type SearchResult struct {
	Files       []SearchFileResult `json:"files"`       // Files with matches
	TotalFiles  int                `json:"totalFiles"`  // Total number of files with matches
	TotalMatches int               `json:"totalMatches"` // Total number of matches across all files
}

// ReplaceParams contains parameters for replacing text
type ReplaceParams struct {
	WorkspaceID    string `json:"workspaceId"`
	Pattern        string `json:"pattern"`
	Replacement    string `json:"replacement"`
	UseRegex       bool   `json:"useRegex"`
	CaseSensitive  bool   `json:"caseSensitive"`
	WholeWord      bool   `json:"wholeWord"`
	IncludePattern string `json:"includePattern"`
	ExcludePattern string `json:"excludePattern"`
	FilePath       string `json:"filePath"`       // Optional: replace only in this file
	DryRun         bool   `json:"dryRun"`         // If true, return what would be replaced without actually replacing
}

// ReplaceResult represents the result of a replace operation
type ReplaceResult struct {
	FilesModified     int      `json:"filesModified"`     // Number of files modified
	TotalReplacements int      `json:"totalReplacements"` // Total number of replacements made
	ModifiedFiles     []string `json:"modifiedFiles"`     // List of modified file paths
}

// SearchResultWithNodes represents search results mapped to graph nodes
// This allows the frontend to directly use the results without transformation
type SearchResultWithNodes struct {
	ResultFolder *graph.FolderNode `json:"resultFolder"` // Root folder containing all results
	TotalFiles   int               `json:"totalFiles"`   // Total number of files with matches
	TotalMatches int               `json:"totalMatches"` // Total number of matches across all files
}
