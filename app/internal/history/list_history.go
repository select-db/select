package history

import (
	"context"
	"encoding/json"
	"time"

	"selectDb/internal/db/generated"
)

// HistoryEntry is the frontend-facing shape of a recorded statement. The
// database name/type are intentionally omitted: the frontend resolves them
// from the workspace graph via DbInstanceID so renamed/removed instances stay
// in sync instead of showing a stale snapshot.
type HistoryEntry struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	AffectedRows *int64   `json:"affectedRows"`
	RowCount     *int64   `json:"rowCount"`
	DurationMs   *int64   `json:"durationMs"`
	Errors       []string `json:"errors"`
	WorkspaceID  string   `json:"workspaceId"`
	DbInstanceID string   `json:"dbInstanceId"`
	CreatedAt    string   `json:"createdAt"`
}

type ListHistoryParams struct {
	WorkspaceID string `json:"workspaceId"`
	Limit       int    `json:"limit"`
	Offset      int    `json:"offset"`
}

const defaultHistoryPageSize = 50

// ListHistory returns recorded statements for a workspace from the last 7 days,
// newest first, paginated by limit/offset.
func (h *History) ListHistory(params ListHistoryParams) ([]HistoryEntry, error) {
	ctx := context.Background()

	limit := params.Limit
	if limit <= 0 {
		limit = defaultHistoryPageSize
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := h.Queries.ListHistory(ctx, generated.ListHistoryParams{
		WorkspaceID: params.WorkspaceID,
		Limit:       int64(limit),
		Offset:      int64(offset),
	})
	if err != nil {
		return nil, err
	}

	entries := make([]HistoryEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toHistoryEntry(row))
	}
	return entries, nil
}

// PruneOnStartup clears stale history once at launch: rows older than 7 days
// and anything beyond the newest 100 per workspace. Best-effort and silent.
func (h *History) PruneOnStartup() {
	ctx := context.Background()
	_ = h.Queries.DeleteHistoryOlderThan7Days(ctx)
	_ = h.Queries.PruneHistoryAllWorkspaces(ctx)
}

func toHistoryEntry(row generated.History) HistoryEntry {
	entry := HistoryEntry{
		ID:           row.ID,
		Statement:    row.Statement,
		WorkspaceID:  row.WorkspaceID,
		DbInstanceID: row.DbInstanceID,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
		Errors:       []string{},
	}

	if row.AffectedRows.Valid {
		v := row.AffectedRows.Int64
		entry.AffectedRows = &v
	}
	if row.RowCount.Valid {
		v := row.RowCount.Int64
		entry.RowCount = &v
	}
	if row.DurationMs.Valid {
		v := row.DurationMs.Int64
		entry.DurationMs = &v
	}
	if row.Errors != "" {
		var errs []string
		if err := json.Unmarshal([]byte(row.Errors), &errs); err == nil && errs != nil {
			entry.Errors = errs
		}
	}

	return entry
}
