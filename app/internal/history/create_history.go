package history

import (
	"context"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

type CreateQueryHistoryParams struct {
	Dsn string
	Uri string

	WorkspaceID  string
	DbInstanceID string

	Statement    string
	AffectedRows *int32
	RowCount     *int32
	DurationMs   *int32
	Errors       []string
}

func (h *History) CreateHistory(params CreateQueryHistoryParams) error {
	ctx := context.Background()
	ID := utils.GenerateUUID()

	errorsJSON, err := utils.ToJSONString(params.Errors)
	if err != nil {
		return err
	}

	historyParams := generated.CreateHistoryParams{
		ID:  ID,
		Dsn: params.Dsn,
		Uri: params.Uri,

		WorkspaceID:  params.WorkspaceID,
		DbInstanceID: params.DbInstanceID,

		Statement:    params.Statement,
		AffectedRows: utils.ToNullInt64(params.AffectedRows),
		RowCount:     utils.ToNullInt64(params.RowCount),
		DurationMs:   utils.ToNullInt64(params.DurationMs),
		Errors:       errorsJSON,
	}

	if _, err = h.Queries.CreateHistory(ctx, historyParams); err != nil {
		return err
	}

	// Best-effort retention. History is a local convenience log, so a failed
	// prune must never fail the write: keep the newest 100 for this workspace
	// and drop anything older than 7 days (the panel only shows that window).
	_ = h.Queries.PruneHistoryForWorkspace(ctx, params.WorkspaceID)
	_ = h.Queries.DeleteHistoryOlderThan7Days(ctx)

	return nil
}
