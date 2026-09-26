package db_client

import (
	"fmt"

	"selectDb/internal/graph"
	"selectDb/internal/utils"

	"github.com/selectDb/dialect/core"
)

type PlanParams struct {
	FileID       string
	Statement    string
	DatasourceID string
	FolderID     string
	RuntimeVars  map[string]string
}

func (dbc *DbClient) Plan(params PlanParams) graph.ExplainResult {
	var result graph.ExplainResult
	if id, err := utils.GenerateRandomID(4); err == nil {
		result.Id = id
	}

	datasource := dbc.Graph.GetDatasourceNodeByID(params.DatasourceID)
	if datasource == nil {
		result.Errors = []string{fmt.Sprintf("failed to get datasource with id: %s", params.DatasourceID)}
		return result
	}

	parser, err := core.NewPlanParser(datasource.DBType)
	if err != nil {
		result.Errors = []string{fmt.Sprintf("plan not supported for %s: %v", datasource.DBType, err)}
		return result
	}

	engineResult, _ := dbc.execute(executeParams{
		DatasourceID: params.DatasourceID,
		FileID:       params.FileID,
		Statement:    parser.BuildPlanQuery(params.Statement),
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
		result.Errors = []string{fmt.Sprintf("failed to parse plan output: %v", err)}
		return result
	}

	root, total := core.BuildVisualTreeForPlan(plan, parser)
	result.Root = root
	if total > 0 {
		result.TotalCost = utils.Ptr(total)
	}

	return result
}
