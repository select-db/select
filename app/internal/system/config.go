package system

import (
	"time"

	"selectDb/internal/graph"
	"selectDb/internal/utils"
)

// GetConfig loads and returns the merged personal config (defaults + per-user
// .config keybindings/snippets).
func (s *System) GetConfig() (*graph.ConfigResponse, error) {
	return s.Graph.LoadConfig()
}

// ResetUserConfig resets the per-user .config (keybindings/snippets) to defaults.
func (s *System) ResetUserConfig() error {
	return graph.ResetUserConfig()
}

// GetDefaultUserConfigContent returns the default per-user .config content
// (keybindings/snippets). Used by the personal config editor.
func (s *System) GetDefaultUserConfigContent() string {
	return graph.GetDefaultUserConfigContent()
}

// EnsureUserConfigDefaults makes sure the per-user .theme and .config files
// exist on disk, seeding the built-in defaults when missing (existing files are
// preserved). Called before opening the personal Settings editors so they can
// always read the files, even if seeding at startup failed or a file was
// deleted.
func (s *System) EnsureUserConfigDefaults() error {
	dir, err := graph.UserConfigDir()
	if err != nil {
		return err
	}
	return graph.SeedUserDefaultFiles(dir)
}

// ExecutionLimitsResponse is the workspace-level query safety policy returned to
// the frontend after an update.
type ExecutionLimitsResponse struct {
	StatementTimeoutMs int `json:"statement_timeout_ms"`
	MaxResultSizeMB    int `json:"max_result_size_mb"`
}

// UpdateWorkspaceExecutionLimits persists the workspace execution limits (a
// tracked write that syncs to the team) and returns the normalized values that
// were stored. Emits workspaceGraphUpdated so the UI reflects the change.
func (s *System) UpdateWorkspaceExecutionLimits(statementTimeoutMs, maxResultSizeMB int) (*ExecutionLimitsResponse, error) {
	timeoutMs, sizeMB, err := s.Graph.UpdateWorkspaceExecutionLimits(statementTimeoutMs, maxResultSizeMB)
	if err != nil {
		return nil, err
	}
	if ws, err := s.Graph.GetWorkspaceGraph(); err == nil {
		utils.DebouncedEventsEmit("workspaceGraphUpdated", 100*time.Millisecond, ws)
	}
	return &ExecutionLimitsResponse{StatementTimeoutMs: timeoutMs, MaxResultSizeMB: sizeMB}, nil
}
