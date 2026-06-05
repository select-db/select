package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"backend/internal/api/datasource"
	"backend/internal/authz"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

const (
	defaultMaxRows = 50
	maxRowsCeiling = 250
)

// ----------------------------------------------------------------------
// execute_query
// ----------------------------------------------------------------------

func toolExecuteQuery() Tool {
	return Tool{
		Name: "execute_query",
		Description: "Executes a read-only SQL statement against a datasource and returns up to max_rows rows " +
			"(default 50, cap 250). Use this for SELECTs and other read-only queries. " +
			"For writes, use execute_statement. Before calling, verify tables with get_database_schemas " +
			"and get_database_table_detail.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
			"statement":     stringProp("SQL SELECT statement to execute (required)."),
			"max_rows":      intProp("Row cap (default 50, max 250)."),
		}, []string{"datasource_id", "statement"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint: boolPtr(true),
		},
		Run: func(ctx context.Context, r *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string `json:"datasource_id"`
				Statement    string `json:"statement"`
				MaxRows      int    `json:"max_rows"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.DatasourceID == "" || args.Statement == "" {
				return nil, errBadArgument("datasource_id and statement are required")
			}
			maxRows := args.MaxRows
			if maxRows <= 0 {
				maxRows = defaultMaxRows
			}
			if maxRows > maxRowsCeiling {
				maxRows = maxRowsCeiling
			}

			conn, ds, _, err := openConn(ctx, r, args.DatasourceID, workspaceID)
			if err != nil {
				return nil, err
			}
			return runQuery(ctx, conn, ds, args.Statement, maxRows), nil
		},
	}
}

// ----------------------------------------------------------------------
// execute_statement
// ----------------------------------------------------------------------

func toolExecuteStatement() Tool {
	return Tool{
		Name: "execute_statement",
		Description: "Executes a write or DDL SQL statement (INSERT, UPDATE, DELETE, MERGE, CREATE, ALTER, DROP, " +
			"TRUNCATE) against a datasource. The role on the calling API key must permit writes on the target " +
			"datasource; otherwise the engine rejects the statement.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
			"statement":     stringProp("SQL statement to execute (required)."),
		}, []string{"datasource_id", "statement"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint:    boolPtr(false),
			DestructiveHint: boolPtr(true),
		},
		Run: func(ctx context.Context, r *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string `json:"datasource_id"`
				Statement    string `json:"statement"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.DatasourceID == "" || args.Statement == "" {
				return nil, errBadArgument("datasource_id and statement are required")
			}

			conn, ds, _, err := openConn(ctx, r, args.DatasourceID, workspaceID)
			if err != nil {
				return nil, err
			}
			return runQuery(ctx, conn, ds, args.Statement, 0 /* no row cap; usually no rows for DML/DDL */), nil
		},
	}
}

// ----------------------------------------------------------------------
// shared
// ----------------------------------------------------------------------

func openConn(ctx context.Context, r *http.Request, datasourceID, workspaceID string) (engine.Conn, *datasource.ResolvedDatasource, core.SQLDialect, error) {
	ds, err := datasource.GetOrLoadDatasource(ctx, datasourceID, workspaceID)
	if err != nil {
		return engine.Conn{}, nil, nil, errNotFound("datasource not found")
	}
	dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
	if err != nil {
		return engine.Conn{}, nil, nil, errUpstream("could not open datasource connection: " + err.Error())
	}
	dialect := engine.GetDialect(ds.DBType)
	if dialect == nil {
		return engine.Conn{}, nil, nil, errExecution("unsupported database type: "+ds.DBType, "")
	}
	meta, err := engine.GetOrFetchMetadata(ctx, workspaceID, ds.DSN, dbConn, dialect, "", false)
	if err != nil {
		return engine.Conn{}, nil, nil, errUpstream("could not fetch metadata: " + err.Error())
	}
	return engine.Conn{
		DB:   dbConn,
		Meta: meta,
		// Default-deny: no explicit allow rules = no access
		Perms: authz.CompiledFromRequest(r).WithDenyUnmanaged(),
	}, ds, dialect, nil
}

func runQuery(ctx context.Context, conn engine.Conn, ds *datasource.ResolvedDatasource, sql string, maxRows int) any {
	sink := newCollectSink(maxRows)
	inst := engine.DBInstance{ID: "mcp", DBType: ds.DBType}
	engine.StreamLocal(ctx, conn, inst, sql, engine.Options{
		// Bound the work even when callers don't supply a timeout.
		Timeout:  30 * time.Second,
		MaxRows:  maxRows,
		MaxBytes: 8 * 1024 * 1024, // 8MB hard cap for MCP results
	}, sink)
	return sink.Result()
}

type collectSink struct {
	maxRows   int
	columns   []string
	rows      [][]any
	rowCount  int64
	affected  int64
	duration  int64
	err       error
	truncated bool
}

func newCollectSink(maxRows int) *collectSink {
	return &collectSink{maxRows: maxRows}
}

func (c *collectSink) OnColumns(cols []string) error {
	c.columns = cols
	return nil
}

func (c *collectSink) OnRow(values []any) error {
	// StreamLocal reuses backing slices
	row := make([]any, len(values))
	copy(row, values)
	c.rows = append(c.rows, row)
	return nil
}

func (c *collectSink) OnDone(rowCount, affected, durationMs int64) error {
	c.rowCount = rowCount
	c.affected = affected
	c.duration = durationMs
	return nil
}

func (c *collectSink) OnTruncated() {
	c.truncated = true
}

func (c *collectSink) OnError(err error) {
	c.err = err
}

func (c *collectSink) Result() any {
	if c.err != nil {
		return map[string]any{
			"success":     false,
			"error":       c.err.Error(),
			"duration_ms": c.duration,
		}
	}
	out := map[string]any{
		"success":     true,
		"columns":     c.columns,
		"rows":        c.rows,
		"row_count":   c.rowCount,
		"duration_ms": c.duration,
	}
	if c.affected > 0 {
		out["affected_rows"] = c.affected
	}
	if c.truncated {
		out["truncated"] = true
	}
	return out
}
