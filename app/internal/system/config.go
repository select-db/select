package system

import (
	"selectDb/internal/graph"
)

// GetConfig loads and returns the merged config across defaults -> workspace
// (execution limits) -> user (keybindings/snippets).
func (s *System) GetConfig() (*graph.ConfigResponse, error) {
	return s.Graph.LoadWorkspaceConfig()
}

// ResetConfig resets the workspace .config (execution limits) to defaults.
func (s *System) ResetConfig() error {
	return s.Graph.ResetWorkspaceConfig()
}

// ResetUserConfig resets the per-user .config (keybindings/snippets) to defaults.
func (s *System) ResetUserConfig() error {
	return graph.ResetUserConfig()
}

// GetDefaultConfigContent returns the default workspace .config content
// (execution limits). Kept for the workspace config editor.
func (s *System) GetDefaultConfigContent() string {
	return graph.GetDefaultWorkspaceConfigContent()
}

// GetDefaultUserConfigContent returns the default per-user .config content
// (keybindings/snippets). Used by the personal config editor.
func (s *System) GetDefaultUserConfigContent() string {
	return graph.GetDefaultUserConfigContent()
}
