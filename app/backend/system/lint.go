package system

import (
	"selectDb/backend/graph"

	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

// GetLint loads and returns the workspace .lint config entries.
func (s *System) GetLint() (ta.LintFile, error) {
	return s.Graph.LoadWorkspaceLint()
}

// ResetLint writes the default .lint content to the workspace file.
func (s *System) ResetLint() error {
	return s.Graph.ResetWorkspaceLint()
}

// GetDefaultLintContent returns the default .lint file content.
func (s *System) GetDefaultLintContent() string {
	return graph.GetDefaultLintContent()
}
