package db_client

import (
	"context"
	"fmt"
	"time"

	"selectDb/internal/graph"

	"github.com/selectDb/dialect/engine"
)

type PingParams struct {
	DbInstanceID string                     `json:"DbInstanceID"`
	DbType       string                     `json:"db_type"`
	Dsn          string                     `json:"dsn"`
	FolderId     string                     `json:"folder_id"`
	Ssh          *graph.DBInstanceSSHConfig `json:"ssh,omitempty"`
	Proxified    bool                       `json:"proxified"`
	NoCache      bool                       `json:"no_cache,omitempty"`
}

// Ping checks if the database instance is reachable, and reports what it found.
//
// A ping has no other purpose, so the report is deferred rather than left to
// the caller: no return path can be added that forgets to say what it learned.
func (dbc *DbClient) Ping(params PingParams) (result string) {
	defer func() { emitAvailability(params.DbInstanceID, result) }()

	base := dbc.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()

	openWorkspaceID, _, _ := graph.OpenWorkspace()

	node := &graph.DBInstanceNode{
		ID:          params.DbInstanceID,
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

	inst := engine.DBInstance{
		ID:        node.ID,
		DBType:    node.DBType,
		Proxified: node.Proxified,
	}
	if err := engineClient.Ping(ctx, conn, inst, node.WorkspaceID, params.NoCache); err != nil {
		return err.Error()
	}
	return ""
}
