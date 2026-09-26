package db_client

import (
	"fmt"

	"selectDb/internal/graph"
	"selectDb/internal/utils"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/dialects"
)

// LookupForeignKeyParams is the payload from the frontend's FK picker. It is
// deliberately structured (no SQL strings on the wire) so the dialect-specific
// SQL building happens entirely in the corresponding dialect package.
type LookupForeignKeyParams struct {
	DatasourceID   string   `json:"datasourceId"`
	Schema         string   `json:"schema"`
	Table          string   `json:"table"`
	FKColumn       string   `json:"fkColumn"`
	DisplayColumns []string `json:"displayColumns"`
	Query          string   `json:"query"`
	CurrentValue   string   `json:"currentValue"`
	Limit          int      `json:"limit"`
}

// LookupForeignKey runs a dialect-aware SELECT against the FK target table and
// returns the rows the picker should show. The frontend never sees the SQL.
func (dbc *DbClient) LookupForeignKey(params LookupForeignKeyParams) graph.QueryResult {
	var result graph.QueryResult
	if id, err := utils.GenerateRandomID(4); err == nil {
		result.Id = id
	}

	datasource := dbc.Graph.GetDatasourceNodeByID(params.DatasourceID)
	if datasource == nil {
		result.Errors = []string{"database not found"}
		return result
	}

	dialect := dialects.Get(datasource.DBType)
	if dialect == nil {
		result.Errors = []string{fmt.Sprintf("unsupported DB type for FK lookup: %s", datasource.DBType)}
		return result
	}

	sql := dialect.BuildForeignKeyLookupSQL(core.ForeignKeyLookupParams{
		Schema:         params.Schema,
		Table:          params.Table,
		FKColumn:       params.FKColumn,
		DisplayColumns: params.DisplayColumns,
		Query:          params.Query,
		CurrentValue:   params.CurrentValue,
		Limit:          params.Limit,
	})

	engineResult, _ := dbc.execute(executeParams{
		DatasourceID: params.DatasourceID,
		Statement:    sql,
		// No file context: the picker query is incidental, not tracked.
		ForExport: true,
	})

	result.Columns = engineResult.Columns
	result.Rows = engineResult.Rows
	result.RowCount = engineResult.RowCount
	result.DurationMs = engineResult.DurationMs
	if len(engineResult.Errors) > 0 {
		result.Errors = engineResult.Errors
	}
	return result
}
