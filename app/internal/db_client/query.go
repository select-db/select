package db_client

import (
	"context"
	"time"

	"selectDb/internal/graph"
	"selectDb/internal/utils"

	"github.com/selectDb/dialect/engine"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type QueryParams struct {
	FileID       string
	Statement    string
	DbInstanceID string
	FolderID     string
	ForExport    bool
	RuntimeVars  map[string]string
}

// PageSize is the number of rows the frontend pages by. Kept in sync with the
// virtualized table's expectations.
const PageSize = 75

// Query is the legacy synchronous entry point. It blocks until all rows are
// available, then returns the full Result. Used by Export and any call site
// that explicitly needs ForExport semantics.
//
// Interactive callers should prefer StartQuery, which streams progress and
// hands back rows page-by-page from the streaming cache.
func (dbc *DbClient) Query(params QueryParams) graph.QueryResult {
	var result graph.QueryResult
	if id, err := utils.GenerateRandomID(4); err == nil {
		result.Id = id
	}

	engineResult, _ := dbc.execute(executeParams{
		DbInstanceID: params.DbInstanceID,
		FileID:       params.FileID,
		Statement:    params.Statement,
		FolderID:     params.FolderID,
		RuntimeVars:  params.RuntimeVars,
		ForExport:    params.ForExport,
	})

	result.Columns = engineResult.Columns
	result.RowCount = engineResult.RowCount
	result.DurationMs = engineResult.DurationMs
	result.AffectedRows = engineResult.AffectedRows
	result.ErrorPosition = engineResult.ErrorPosition
	result.ColumnMetadata = columnEditMetaToGraph(engineResult.ColumnEditMeta)
	result.Rows = engineResult.Rows
	result.PageSize = engineResult.RowCount
	if len(engineResult.Errors) > 0 {
		result.Errors = engineResult.Errors
	}
	return result
}

// StartQueryParams kicks off a streaming query. The returned ExecutionID is the
// key used to correlate Wails events ("query:started", "query:progress",
// "query:done", "query:error") and to fetch pages via GetResultPage.
type StartQueryParams struct {
	FileID       string
	Statement    string
	DbInstanceID string
	FolderID     string
	RuntimeVars  map[string]string
}

// StartQueryResult is returned synchronously from StartQuery. The query runs
// in a background goroutine; results arrive via Wails events.
type StartQueryResult struct {
	ExecutionID  string   `json:"executionId"`
	DbInstanceID string   `json:"dbInstanceId"`
	FileID       string   `json:"fileId"`
	Errors       []string `json:"errors,omitempty"`
}

// StartQuery validates inputs and kicks off a streaming execution.
// Emits Wails events keyed by executionId; returns immediately.
func (dbc *DbClient) StartQuery(params StartQueryParams) StartQueryResult {
	executionID, _ := utils.GenerateRandomID(4)

	out := StartQueryResult{
		ExecutionID:  executionID,
		DbInstanceID: params.DbInstanceID,
		FileID:       params.FileID,
	}

	p, err := dbc.prepare(executeParams{
		DbInstanceID: params.DbInstanceID,
		FileID:       params.FileID,
		Statement:    params.Statement,
		FolderID:     params.FolderID,
		RuntimeVars:  params.RuntimeVars,
	})
	if err != nil {
		out.Errors = []string{err.Error()}
		// Emit an immediate error event so the frontend can surface it
		// through the same event channel as live failures.
		runtime.EventsEmit(dbc.ctx, "query:error", queryErrorEvent{
			ExecutionID:  executionID,
			DbInstanceID: params.DbInstanceID,
			FileID:       params.FileID,
			Message:      err.Error(),
		})
		return out
	}

	// HTTP context gets +10s margin so the backend's DB timeout fires first.
	ctx, cancel := context.WithTimeout(dbc.ctx, p.timeout+10*time.Second)

	listener := &queryEventListener{
		ctx:          dbc.ctx,
		executionID:  executionID,
		dbInstanceID: params.DbInstanceID,
		fileID:       params.FileID,
		dbInstance:   p.dbInstance,
		statement:    p.statement,
		dbc:          dbc,
		cancelTimer:  cancel,
	}

	engineClient.Stream(
		ctx,
		queryKey(params.DbInstanceID, params.FileID),
		p.conn,
		p.instance,
		p.dbInstance.WorkspaceID,
		p.statement,
		engine.Options{
			MaxBytes: p.maxBytes,
			Timeout:  p.timeout,
		},
		listener,
	)

	runtime.EventsEmit(
		dbc.ctx,
		"databaseAvailability", map[string]interface{}{
			"databases": []map[string]interface{}{
				{"id": params.DbInstanceID},
			},
		})

	return out
}

type GetResultPageParams struct {
	DbInstanceID string
	FileID       string
	ResultID     string
	Page         int
}

// GetResultPage returns a page from the streaming cache. The returned Status
// indicates whether the page is "ready", "partial" (fewer rows than pageSize
// because the stream is still filling), or "pending" (no rows yet for this
// window).
func (dbc *DbClient) GetResultPage(params GetResultPageParams) graph.QueryResult {
	var queryResult graph.QueryResult
	queryResult.Id = params.ResultID

	page, status, ok := engineClient.Page(
		queryKey(params.DbInstanceID, params.FileID),
		params.Page,
		PageSize,
	)
	if !ok {
		queryResult.Errors = []string{"Result not found in cache or expired. Please re-run the query."}
		queryResult.Status = pageStatusString(PageStatusReady)
		return queryResult
	}

	queryResult.Columns = page.Columns
	queryResult.Rows = page.Rows
	queryResult.RowCount = int(page.RowCount)
	queryResult.AffectedRows = page.AffectedRows
	queryResult.DurationMs = page.DurationMs
	queryResult.Available = int(page.Available)
	queryResult.Page = page.Page
	queryResult.PageSize = page.PageSize
	queryResult.ColumnMetadata = columnEditMetaToGraph(page.ColumnEditMeta)
	queryResult.ErrorPosition = page.ErrorPosition
	queryResult.Status = pageStatusString(toPageStatusKind(status))

	if len(page.Errors) > 0 {
		queryResult.Errors = page.Errors
	}

	return queryResult
}

// PageStatusKind mirrors engine.PageStatus on the wire.
type PageStatusKind int

const (
	PageStatusReady PageStatusKind = iota
	PageStatusPartial
	PageStatusPending
)

func pageStatusString(k PageStatusKind) string {
	switch k {
	case PageStatusPartial:
		return "partial"
	case PageStatusPending:
		return "pending"
	default:
		return "ready"
	}
}

func toPageStatusKind(s engine.PageStatus) PageStatusKind {
	switch s {
	case engine.PagePartial:
		return PageStatusPartial
	case engine.PagePending:
		return PageStatusPending
	default:
		return PageStatusReady
	}
}
