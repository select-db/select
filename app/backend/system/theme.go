package system

import (
	"selectDb/backend/graph"
)

// GetThemeVariables loads and returns the current workspace theme variables.
// Called by the frontend on startup to initialize the theme.
// When there is no workspace (e.g. login page), returns the default theme so the app is still styled.
func (s *System) GetThemeVariables() (*graph.ThemeVariables, error) {
	vars, err := s.Graph.LoadWorkspaceTheme()
	if err != nil {
		return graph.LoadDefaultTheme(), nil
	}
	return vars, nil
}

// ResetTheme resets the workspace theme to default values by writing
// the default theme content to the .theme file.
func (s *System) ResetTheme() error {
	return s.Graph.ResetWorkspaceTheme()
}

// GetDefaultThemeContent returns the default theme file content.
// Used when creating a new .theme file or showing users the template.
func (s *System) GetDefaultThemeContent() string {
	return graph.GetDefaultThemeContent()
}
