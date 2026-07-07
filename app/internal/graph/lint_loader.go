package graph

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

//go:embed defaults/workspace/.lint
var DefaultLintContent string

const LintFileName = ".lint"

// GetDefaultLintContent returns the default lint file content.
func GetDefaultLintContent() string {
	return DefaultLintContent
}

// GetLintFilePath returns the path to the .lint file for the current workspace.
func (g *Graph) GetLintFilePath() (string, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return "", fmt.Errorf("workspace graph not initialized: %w", err)
	}
	wfs, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create workspace fs: %w", err)
	}
	return filepath.Join(wfs.WorkspaceRoot, LintFileName), nil
}

// LoadWorkspaceLint reads and parses the workspace .lint file.
func (g *Graph) LoadWorkspaceLint() (ta.LintFile, error) {
	lintPath, err := g.GetLintFilePath()
	if err != nil {
		return ta.LintFile{}, nil
	}
	data, err := os.ReadFile(lintPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ta.LintFile{}, nil
		}
		return nil, fmt.Errorf("failed to read .lint file: %w", err)
	}
	return ta.ParseLintConfig(string(data))
}

// ResetWorkspaceLint writes the default .lint content to the workspace file.
func (g *Graph) ResetWorkspaceLint() error {
	lintPath, err := g.GetLintFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(lintPath, []byte(DefaultLintContent), 0644)
}
