package system

import (
	"selectDb/backend/graph"
)

// GetConfig loads and returns the merged config (defaults + user .config).
func (s *System) GetConfig() (*graph.ConfigResponse, error) {
	return s.Graph.LoadWorkspaceConfig()
}

// Resets the workspace config to default values by writing
func (s *System) ResetConfig() error {
	return s.Graph.ResetWorkspaceConfig()
}

// GetDefaultConfigContent returns the default config file content.
func (s *System) GetDefaultConfigContent() string {
	return graph.GetDefaultConfigContent()
}
