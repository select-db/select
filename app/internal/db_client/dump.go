package db_client

import (
	"selectDb/internal/graph"
	"selectDb/internal/sqllang"

	"github.com/selectDb/dialect/engine/connect"
)

// effectiveDSN resolves variables and rewrites host:port to the SSH tunnel
// local listener when enabled, so CLI tools (pg_dump, mysqldump) connect
// the same way as the in-process sql.DB.
func (dbc *DbClient) effectiveDSN(dbInstance *graph.DBInstanceNode) string {
	dsn, err := sqllang.SubstituteVariables(dbc.Graph, dbInstance.DSN, dbInstance.FolderID)
	if err != nil {
		return dbInstance.DSN
	}
	if dbInstance.SSH == nil || !dbInstance.SSH.Enabled {
		return dsn
	}
	resolvedSSH, err := dbc.resolveSSHConfig(dbInstance.SSH, dbInstance.FolderID)
	if err != nil || resolvedSSH == nil {
		return dsn
	}
	tunneled, err := connect.ResolveDumpDSN(dbInstance.WorkspaceID, dbInstance.DBType, dsn, resolvedSSH)
	if err != nil {
		return dsn
	}
	return tunneled
}
