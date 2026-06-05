package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/selectDb/dialect/core"
)

// ----------------------------------------------------------------------
// plan_query: non-executing (EXPLAIN without ANALYZE).
// ----------------------------------------------------------------------

func toolPlanQuery() Tool {
	return Tool{
		Name: "plan_query",
		Description: "Generates a query execution plan by parsing the query without executing it " +
			"(equivalent to EXPLAIN without ANALYZE). No data is read or modified.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
			"statement":     stringProp("SQL statement to plan (required)."),
		}, []string{"datasource_id", "statement"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint:   boolPtr(true),
			IdempotentHint: boolPtr(true),
		},
		Run: func(ctx context.Context, r *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			return runPlanOrExplain(ctx, r, workspaceID, raw, true)
		},
	}
}

// ----------------------------------------------------------------------
// explain_query: executes the query and returns its plan (EXPLAIN ANALYZE).
// ----------------------------------------------------------------------

func toolExplainQuery() Tool {
	return Tool{
		Name: "explain_query",
		Description: "Executes a read-only SQL statement under EXPLAIN ANALYZE and returns the actual plan. " +
			"Only pass SELECTs or other read-only statements: this tool executes the statement to collect real " +
			"runtime stats, so passing a write would run the write. For writes, use execute_statement. " +
			"For the planner's view without execution, use plan_query.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
			"statement":     stringProp("Read-only SQL statement to explain (required)."),
		}, []string{"datasource_id", "statement"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint: boolPtr(true),
		},
		Run: func(ctx context.Context, r *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			return runPlanOrExplain(ctx, r, workspaceID, raw, false)
		},
	}
}

// runPlanOrExplain shares the wrapper logic between plan_query (parser
// only) and explain_query (executor). The dialect package decides what
// SQL to actually emit.
func runPlanOrExplain(ctx context.Context, r *http.Request, workspaceID string, raw json.RawMessage, planOnly bool) (any, error) {
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

	conn, ds, dialect, err := openConn(ctx, r, args.DatasourceID, workspaceID)
	if err != nil {
		return nil, err
	}

	var (
		wrappedSQL string
		parser     core.ExplainParser
		planParser core.PlanParser
	)
	if planOnly {
		planParser, err = core.NewPlanParser(dialect.Name())
		if err != nil {
			return nil, errExecution(fmt.Sprintf("plan not supported for %s", dialect.Name()), "")
		}
		wrappedSQL = planParser.BuildPlanQuery(args.Statement)
	} else {
		parser, err = core.NewExplainParser(dialect.Name())
		if err != nil {
			return nil, errExecution(fmt.Sprintf("explain not supported for %s", dialect.Name()), "")
		}
		wrappedSQL = parser.BuildExplainQuery(args.Statement)
	}

	res := runQuery(ctx, conn, ds, wrappedSQL, maxRowsCeiling).(map[string]any)
	if ok, _ := res["success"].(bool); !ok {
		return res, nil
	}

	// Convert rows into the raw text form parsers expect.
	rows, _ := res["rows"].([][]any)
	rawText := rowsToRawText(rows)

	var rootNode any
	var totalCost any
	var parseErr error
	if planOnly {
		node, perr := planParser.Parse(rawText)
		parseErr = perr
		if perr == nil {
			tree, total := core.BuildVisualTreeForPlan(node, planParser)
			rootNode = tree
			if total > 0 {
				totalCost = total
			}
		}
	} else {
		node, perr := parser.Parse(rawText)
		parseErr = perr
		if perr == nil {
			tree, total := core.BuildVisualTree(node, parser)
			rootNode = tree
			if total > 0 {
				totalCost = total
			}
		}
	}

	out := map[string]any{
		"success":     true,
		"raw":         rawText,
		"duration_ms": res["duration_ms"],
	}
	if parseErr == nil {
		out["plan"] = rootNode
		if totalCost != nil {
			out["total_cost"] = totalCost
		}
	} else {
		// Plan/explain produced output but it didn't parse. Surface the raw
		// text so the model can still use it.
		out["parse_warning"] = parseErr.Error()
	}
	return out, nil
}

func rowsToRawText(rows [][]any) string {
	if len(rows) == 0 {
		return ""
	}
	out := make([]byte, 0, 64*len(rows))
	for ri, row := range rows {
		if ri > 0 {
			out = append(out, '\n')
		}
		for i, v := range row {
			if i > 0 {
				out = append(out, '\t')
			}
			switch s := v.(type) {
			case nil:
				// empty cell
			case []byte:
				out = append(out, s...)
			case string:
				out = append(out, s...)
			default:
				out = append(out, []byte(fmt.Sprintf("%v", v))...)
			}
		}
	}
	return string(out)
}
