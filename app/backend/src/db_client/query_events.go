package db_client

import (
	"context"

	"selectDb/backend/graph"

	"github.com/selectDb/dialect/engine"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// queryStartedEvent is the payload of "query:started". The frontend uses it
// to materialize a QueryResult shell (id, columns, columnMetadata) before
// any rows arrive.
type queryStartedEvent struct {
	ExecutionID    string                 `json:"executionId"`
	DbInstanceID   string                 `json:"dbInstanceId"`
	FileID         string                 `json:"fileId"`
	Columns        []string               `json:"columns"`
	ColumnMetadata []graph.ColumnMetadata `json:"columnMetadata"`
}

// queryProgressEvent is the payload of "query:progress". Reports the current
// row watermark (rows safe to fetch from the cache, indices [0, available)).
type queryProgressEvent struct {
	ExecutionID  string `json:"executionId"`
	DbInstanceID string `json:"dbInstanceId"`
	FileID       string `json:"fileId"`
	Available    int64  `json:"available"`
}

// queryExecutedEvent is the payload of "query:executed". Surfaces the SQL
// execution duration before all rows have streamed so the UI can stop the
// elapsed-time counter as soon as the actual SQL is done.
type queryExecutedEvent struct {
	ExecutionID  string `json:"executionId"`
	DbInstanceID string `json:"dbInstanceId"`
	FileID       string `json:"fileId"`
	DurationMs   int64  `json:"durationMs"`
}

// queryDoneEvent is the payload of "query:done".
type queryDoneEvent struct {
	ExecutionID  string `json:"executionId"`
	DbInstanceID string `json:"dbInstanceId"`
	FileID       string `json:"fileId"`
	RowCount     int64  `json:"rowCount"`
	AffectedRows int64  `json:"affectedRows"`
	DurationMs   int64  `json:"durationMs"`
}

// queryErrorEvent is the payload of "query:error".
type queryErrorEvent struct {
	ExecutionID   string `json:"executionId"`
	DbInstanceID  string `json:"dbInstanceId"`
	FileID        string `json:"fileId"`
	Message       string `json:"message"`
	ErrorPosition *int   `json:"errorPosition,omitempty"`
}

// queryEventListener implements engine.StreamListener and emits Wails events.
// Also responsible for computing column edit metadata once columns are known
// and stashing it on the StreamingResult so subsequent Page() calls expose it.
type queryEventListener struct {
	ctx          context.Context
	executionID  string
	dbInstanceID string
	fileID       string
	dbInstance   *graph.DBInstanceNode
	statement    string
	dbc          *DbClient
	cancelTimer  context.CancelFunc
}

func (l *queryEventListener) OnStart(columns []string, columnEditMeta []engine.ColumnEditMeta) {
	// Compute column edit metadata if the engine didn't already populate it.
	// Done lazily here so the streaming cache exposes it on the very first
	// Page() call and the started event carries it to the frontend.
	if len(columnEditMeta) == 0 && len(columns) > 0 {
		columnEditMeta = l.dbc.computeColumnEditMeta(l.dbInstance, l.statement)
		if len(columnEditMeta) > 0 {
			if sr, ok := engine.GetStreamingResult(queryKey(l.dbInstanceID, l.fileID)); ok {
				sr.SetColumnEditMeta(columnEditMeta)
			}
		}
	}
	runtime.EventsEmit(l.ctx, "query:started", queryStartedEvent{
		ExecutionID:    l.executionID,
		DbInstanceID:   l.dbInstanceID,
		FileID:         l.fileID,
		Columns:        columns,
		ColumnMetadata: columnEditMetaToGraph(columnEditMeta),
	})
}

func (l *queryEventListener) OnExecuted(durationMs int64) {
	runtime.EventsEmit(l.ctx, "query:executed", queryExecutedEvent{
		ExecutionID:  l.executionID,
		DbInstanceID: l.dbInstanceID,
		FileID:       l.fileID,
		DurationMs:   durationMs,
	})
}

func (l *queryEventListener) OnProgress(available int64) {
	runtime.EventsEmit(l.ctx, "query:progress", queryProgressEvent{
		ExecutionID:  l.executionID,
		DbInstanceID: l.dbInstanceID,
		FileID:       l.fileID,
		Available:    available,
	})
}

func (l *queryEventListener) OnDone(rowCount, affected, durationMs int64) {
	if l.cancelTimer != nil {
		l.cancelTimer()
	}
	runtime.EventsEmit(l.ctx, "query:done", queryDoneEvent{
		ExecutionID:  l.executionID,
		DbInstanceID: l.dbInstanceID,
		FileID:       l.fileID,
		RowCount:     rowCount,
		AffectedRows: affected,
		DurationMs:   durationMs,
	})
}

func (l *queryEventListener) OnError(message string, errorPosition *int) {
	if l.cancelTimer != nil {
		l.cancelTimer()
	}
	runtime.EventsEmit(l.ctx, "query:error", queryErrorEvent{
		ExecutionID:   l.executionID,
		DbInstanceID:  l.dbInstanceID,
		FileID:        l.fileID,
		Message:       message,
		ErrorPosition: errorPosition,
	})
}
