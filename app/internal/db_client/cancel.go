package db_client

type CancelQueryParams struct {
	DbInstanceID string
	FileID       string
}

// CancelQuery aborts the in-flight query for the given DB instance and file.
func (dbc *DbClient) CancelQuery(params CancelQueryParams) {
	engineClient.Cancel(queryKey(params.DbInstanceID, params.FileID))
}
