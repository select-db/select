package graph

import (
	"context"
	"fmt"

	"selectDb/internal/db/generated"
)

// Execution limits are workspace-level query safety policy. They live on the
// workspace row (synced to teammates via the backend, like roles/permissions),
// not in a workspace file.
const (
	DefaultStatementTimeoutMs = 30000
	DefaultMaxResultSizeMB    = 100
	MaxMaxResultSizeMB        = 250
)

// NormalizeStatementTimeoutMs applies the default when the stored value is unset
// or invalid.
func NormalizeStatementTimeoutMs(v int) int {
	if v <= 0 {
		return DefaultStatementTimeoutMs
	}
	return v
}

// NormalizeMaxResultSizeMB applies the default/cap to the stored value.
func NormalizeMaxResultSizeMB(v int) int {
	if v <= 0 {
		return DefaultMaxResultSizeMB
	}
	if v > MaxMaxResultSizeMB {
		return MaxMaxResultSizeMB
	}
	return v
}

// WorkspaceExecutionLimits returns the current workspace statement timeout (ms)
// and max result size (MB), falling back to defaults when no workspace is loaded.
func (g *Graph) WorkspaceExecutionLimits() (statementTimeoutMs, maxResultSizeMB int) {
	ws, err := g.GetWorkspaceGraph()
	if err != nil || ws == nil {
		return DefaultStatementTimeoutMs, DefaultMaxResultSizeMB
	}
	return NormalizeStatementTimeoutMs(ws.StatementTimeoutMs), NormalizeMaxResultSizeMB(ws.MaxResultSizeMB)
}

// UpdateWorkspaceExecutionLimits persists new execution limits on the current
// workspace row (a tracked write that syncs to the team) and updates the
// in-memory workspace node so the change takes effect immediately. It returns
// the normalized values that were stored.
func (g *Graph) UpdateWorkspaceExecutionLimits(statementTimeoutMs, maxResultSizeMB int) (int, int, error) {
	ws, err := g.GetWorkspaceGraph()
	if err != nil {
		return 0, 0, fmt.Errorf("workspace graph not initialized: %w", err)
	}

	statementTimeoutMs = NormalizeStatementTimeoutMs(statementTimeoutMs)
	maxResultSizeMB = NormalizeMaxResultSizeMB(maxResultSizeMB)

	if g.Queries != nil {
		if err := g.Queries.UpdateWorkspaceExecutionLimits(context.Background(), generated.UpdateWorkspaceExecutionLimitsParams{
			ID:                 ws.ID,
			StatementTimeoutMs: int64(statementTimeoutMs),
			MaxResultSizeMb:    int64(maxResultSizeMB),
		}); err != nil {
			return 0, 0, fmt.Errorf("update workspace execution limits: %w", err)
		}
	}

	g.mu.Lock()
	if g.WorkspaceGraph != nil {
		g.WorkspaceGraph.StatementTimeoutMs = statementTimeoutMs
		g.WorkspaceGraph.MaxResultSizeMB = maxResultSizeMB
	}
	g.mu.Unlock()

	return statementTimeoutMs, maxResultSizeMB, nil
}
