package db_client

import (
	"context"
	"fmt"
	"time"

	"selectDb/internal/graph"
	"selectDb/internal/role"
	"selectDb/internal/sqllang"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

type executeParams struct {
	DatasourceID string
	FileID       string
	Statement    string
	FolderID     string
	RuntimeVars  map[string]string
	ForExport    bool
}

type prepared struct {
	conn       engine.Conn
	instance   engine.Datasource
	datasource *graph.DatasourceNode
	statement  string
	timeout    time.Duration
	maxBytes   int64
}

// prepare resolves conn / perms / variables for an execution.
// Used by both the blocking path (execute) and the streaming path.
func (dbc *DbClient) prepare(params executeParams) (*prepared, error) {
	datasource := dbc.Graph.GetDatasourceNodeByID(params.DatasourceID)
	if datasource == nil {
		return nil, fmt.Errorf("failed to get datasource with id: %s", params.DatasourceID)
	}

	instance := engine.Datasource{
		ID:        datasource.ID,
		DBType:    datasource.DBType,
		Proxified: datasource.Proxified,
	}

	statement := params.Statement
	if !datasource.Proxified {
		var err error
		statement, err = sqllang.SubstituteVariablesSQL(
			varResolver(params.RuntimeVars, dbc.Graph),
			params.Statement,
			params.FolderID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to substitute variables: %v", err)
		}
	}

	conn, err := dbc.getEngineConn(datasource)
	if err != nil {
		emitAvailability(datasource.ID, err.Error())
		return nil, fmt.Errorf("failed to open DB: %v", err)
	}

	if !datasource.Proxified {
		conn.Meta, _ = dbc.getCachedMetadata(datasource, false)
		if perms, permErr := role.GetMyPermissions(dbc.Queries); permErr == nil {
			conn.Perms = core.Compile(perms)
		}
	}

	return &prepared{
		conn:       conn,
		instance:   instance,
		datasource: datasource,
		statement:  statement,
		timeout:    dbc.getStatementTimeout(),
		maxBytes:   dbc.getMaxResultSizeBytes(),
	}, nil
}

// execute runs the statement to completion and returns a buffered Result.
// Used by callers that need the full row set up front (Export, Explain, Plan).
func (dbc *DbClient) execute(params executeParams) (*engine.Result, *graph.DatasourceNode) {
	p, err := dbc.prepare(params)
	if err != nil {
		return &engine.Result{Errors: []string{err.Error()}}, nil
	}

	// HTTP context gets +10s margin so the backend's DB timeout fires first.
	ctx, cancel := context.WithTimeout(dbc.ctx, p.timeout+10*time.Second)
	defer cancel()

	result := engineClient.Execute(
		ctx,
		p.conn,
		p.instance,
		p.datasource.WorkspaceID,
		p.statement,
		engine.Options{
			ForExport: params.ForExport,
			MaxBytes:  p.maxBytes,
			Timeout:   p.timeout,
		},
	)

	if len(result.Columns) > 0 && len(result.ColumnEditMeta) == 0 {
		result.ColumnEditMeta = dbc.computeColumnEditMeta(p.datasource, p.statement)
	}

	return result, p.datasource
}

// computeColumnEditMeta returns per-column editability metadata for the
// resolved statement. Returns nil when metadata is missing or the statement
// is not a SELECT.
func (dbc *DbClient) computeColumnEditMeta(datasource *graph.DatasourceNode, statement string) []engine.ColumnEditMeta {
	meta, _ := dbc.getCachedMetadata(datasource, false)
	if meta == nil {
		return nil
	}
	dialect := engine.GetDialect(datasource.DBType)
	if dialect == nil {
		return nil
	}
	inspected := engine.Inspect(dialect, meta, statement)
	stmt, ok := engine.FirstSelectStatement(inspected)
	if !ok {
		return nil
	}
	return engine.AnalyzeEditableColumns(meta, stmt, datasource.ID)
}
