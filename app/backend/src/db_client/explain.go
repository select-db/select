package db_client

import (
	"fmt"
	"strings"

	"selectDb/backend/graph"
	"selectDb/backend/utils"

	"github.com/selectDb/dialect/core"
)

type ExplainParams struct {
	FileID       string
	Statement    string
	DbInstanceID string
	FolderID     string
	RuntimeVars  map[string]string
}

func (dbc *DbClient) Explain(params ExplainParams) graph.ExplainResult {
	var result graph.ExplainResult
	if id, err := utils.GenerateRandomID(4); err == nil {
		result.Id = id
	}

	dbInstance := dbc.Graph.GetDBInstanceNodeByID(params.DbInstanceID)
	if dbInstance == nil {
		result.Errors = []string{fmt.Sprintf("failed to get DB instance with id: %s", params.DbInstanceID)}
		return result
	}

	parser, err := core.NewExplainParser(dbInstance.DBType)
	if err != nil {
		result.Errors = []string{fmt.Sprintf("explain not supported for %s: %v", dbInstance.DBType, err)}
		return result
	}

	engineResult, _ := dbc.execute(executeParams{
		DbInstanceID: params.DbInstanceID,
		FileID:       params.FileID,
		Statement:    parser.BuildExplainQuery(params.Statement),
		FolderID:     params.FolderID,
		RuntimeVars:  params.RuntimeVars,
		ForExport:    true,
	})

	result.DurationMs = engineResult.DurationMs
	result.ErrorPosition = engineResult.ErrorPosition

	if len(engineResult.Errors) > 0 {
		result.Errors = engineResult.Errors
		return result
	}

	raw := rowsToRawString(engineResult.Rows)
	result.Raw = raw

	plan, err := parser.Parse(raw)
	if err != nil {
		result.Errors = []string{fmt.Sprintf("failed to parse explain output: %v", err)}
		return result
	}

	root, total := core.BuildVisualTree(plan, parser)
	result.Root = root
	if total > 0 {
		result.TotalCost = utils.Ptr(total)
	}

	return result
}

// rowsToRawString joins result rows into the tab/newline format explain/plan parsers expect.
func rowsToRawString(rows [][]any) string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		parts := make([]string, len(row))
		for i, val := range row {
			switch v := val.(type) {
			case nil:
				parts[i] = ""
			case []byte:
				parts[i] = string(v)
			case string:
				parts[i] = v
			default:
				parts[i] = fmt.Sprintf("%v", v)
			}
		}
		lines = append(lines, strings.Join(parts, "\t"))
	}
	return strings.Join(lines, "\n")
}
