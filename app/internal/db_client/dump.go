package db_client

import (
	"selectDb/internal/graph"
	"selectDb/internal/sqllang"

	"github.com/selectDb/dialect/engine/connect"
)

// effectiveDSN resolves variables and rewrites host:port to the SSH tunnel
// local listener when enabled, so CLI tools (pg_dump, mysqldump) connect
// the same way as the in-process sql.DB.
func (dbc *DbClient) effectiveDSN(datasource *graph.DatasourceNode) string {
	dsn, err := sqllang.SubstituteVariables(dbc.Graph, datasource.DSN, datasource.FolderID)
	if err != nil {
		return datasource.DSN
	}
	if datasource.SSH == nil || !datasource.SSH.Enabled {
		return dsn
	}
	resolvedSSH, err := dbc.resolveSSHConfig(datasource.SSH, datasource.FolderID)
	if err != nil || resolvedSSH == nil {
		return dsn
	}
	tunneled, err := connect.ResolveDumpDSN(datasource.WorkspaceID, datasource.DBType, dsn, resolvedSSH)
	if err != nil {
		return dsn
	}
	return tunneled
}
