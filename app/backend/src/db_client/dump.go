package db_client

import (
	"selectDb/backend/graph"
	"selectDb/backend/src/sqllang"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
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
	remoteHost, remotePort, err := core.ParseDSNRemote(dbInstance.DBType, dsn)
	if err != nil {
		return dsn
	}
	tunnel, err := engine.GetOrCreateTunnel(dbInstance.WorkspaceID, *resolvedSSH, remoteHost, remotePort)
	if err != nil || tunnel == nil {
		return dsn
	}
	localPort, err := tunnel.LocalPort()
	if err != nil {
		return dsn
	}
	rewritten, err := core.RewriteDSNForLocal(dbInstance.DBType, dsn, "127.0.0.1", localPort)
	if err != nil {
		return dsn
	}
	return rewritten
}
