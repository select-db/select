package db_client

import (
	"selectDb/internal/graph"
	"selectDb/internal/sqllang"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
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
	remoteHost, remotePort, err := core.ParseDSNRemote(datasource.DBType, dsn)
	if err != nil {
		return dsn
	}
	tunnel, err := engine.GetOrCreateTunnel(datasource.WorkspaceID, *resolvedSSH, remoteHost, remotePort)
	if err != nil || tunnel == nil {
		return dsn
	}
	localPort, err := tunnel.LocalPort()
	if err != nil {
		return dsn
	}
	rewritten, err := core.RewriteDSNForLocal(datasource.DBType, dsn, "127.0.0.1", localPort)
	if err != nil {
		return dsn
	}
	return rewritten
}
