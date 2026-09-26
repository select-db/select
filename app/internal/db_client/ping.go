package db_client

import (
	"context"
	"fmt"
	"time"

	"selectDb/internal/graph"

	"github.com/selectDb/dialect/engine/query"
)

type PingParams struct {
	DatasourceID string                     `json:"DatasourceID"`
	DbType       string                     `json:"db_type"`
	Dsn          string                     `json:"dsn"`
	FolderId     string                     `json:"folder_id"`
	Ssh          *graph.DatasourceSSHConfig `json:"ssh,omitempty"`
	Proxified    bool                       `json:"proxified"`
	NoCache      bool                       `json:"no_cache,omitempty"`
}

// Ping checks if the datasource is reachable, and reports what it found.
//
// A ping has no other purpose, so the report is deferred rather than left to
// the caller: no return path can be added that forgets to say what it learned.
func (dbc *DbClient) Ping(params PingParams) (result string) {
	defer func() { emitAvailability(params.DatasourceID, result) }()

	base := dbc.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()

	openWorkspaceID, _, _ := graph.OpenWorkspace()

	node := &graph.DatasourceNode{
		ID:          params.DatasourceID,
		WorkspaceID: openWorkspaceID,
		DBType:      params.DbType,
		DSN:         params.Dsn,
		FolderID:    params.FolderId,
		SSH:         params.Ssh,
		Proxified:   params.Proxified,
	}

	conn, err := dbc.getEngineConn(node)
	if err != nil {
		return fmt.Sprintf("Unable to establish a connection with the database.\nError: %v", err)
	}

	inst := query.Datasource{
		ID:        node.ID,
		DBType:    node.DBType,
		Proxified: node.Proxified,
	}
	if err := engineClient.Ping(ctx, conn, inst, node.WorkspaceID, params.NoCache); err != nil {
		return err.Error()
	}
	return ""
}
