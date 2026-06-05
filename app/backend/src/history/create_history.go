package history

import (
	"context"
	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

type CreateQueryHistoryParams struct {
	Dsn string
	Uri string

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

		Statement:    params.Statement,
		AffectedRows: utils.ToNullInt64(params.AffectedRows),
		RowCount:     utils.ToNullInt64(params.RowCount),
		DurationMs:   utils.ToNullInt64(params.DurationMs),
		Errors:       errorsJSON,
	}

	_, err = h.Queries.CreateHistory(ctx, historyParams)
	if err != nil {
		return err
	}

	return nil
}
