package db_client

type CancelQueryParams struct {
	DatasourceID string
	FileID       string
}

// CancelQuery aborts the in-flight query for the given datasource and file.
func (dbc *DbClient) CancelQuery(params CancelQueryParams) {
	engineClient.Cancel(queryKey(params.DatasourceID, params.FileID))
}
