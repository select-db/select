package db_client

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"selectDb/internal/api"
	"selectDb/internal/db/generated"
	fs_provider "selectDb/internal/fs_provider"
	"selectDb/internal/graph"
	"selectDb/internal/sqllang"
	"time"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/transport"
)

var engineClient = engine.Client{
	Transport: &transport.HTTPTransport{
		Fetch: func(
			ctx context.Context,
			method,
			endpoint string,
			payload,
			response any,
		) error {
			return api.Fetch(ctx, method, endpoint, payload, nil, response)
		},
		FetchStream: func(
			ctx context.Context,
			method,
			endpoint string,
			payload any,
		) (io.ReadCloser, error) {
			return api.FetchStream(ctx, method, endpoint, payload)
		},
	},
}

type DbClient struct {
	ctx        context.Context
	Queries    *generated.Queries
	Graph      *graph.Graph
	FSProvider *fs_provider.FSProvider
}

func New(Queries *generated.Queries, Graph *graph.Graph, FSProvider *fs_provider.FSProvider) *DbClient {
	return &DbClient{
		Queries:    Queries,
		Graph:      Graph,
		FSProvider: FSProvider,
	}
}

func (dbc *DbClient) SetContext(ctx context.Context) {
	dbc.ctx = ctx
}

func (dbc *DbClient) GetOrOpenConn(workspaceID, dbType, dsn, folderID string, sshConfig *graph.DBInstanceSSHConfig) (*sql.DB, error) {
	var err error
	dsn, err = sqllang.SubstituteVariables(dbc.Graph, dsn, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in DSN: %w", err)
	}

	resolvedSSH, err := dbc.resolveSSHConfig(sshConfig, folderID)
	if err != nil {
		return nil, fmt.Errorf("SSH config: %w", err)
	}

	return engine.GetOrOpenConn(workspaceID, dbType, dsn, resolvedSSH)
}

// getEngineConn returns Conn with DB for local, empty Conn{} for proxified.
func (dbc *DbClient) getEngineConn(node *graph.DBInstanceNode) (engine.Conn, error) {
	if node.Proxified {
		// Return empty conn, remote will GetOrOpenConn conn
		return engine.Conn{}, nil
	}
	db, err := dbc.GetOrOpenConn(
		node.WorkspaceID,
		node.DBType,
		node.DSN,
		node.FolderID,
		node.SSH,
	)
	if err != nil {
		return engine.Conn{}, err
	}
	return engine.Conn{DB: db}, nil
}

// getStatementTimeout returns the workspace statement_timeout_ms. Falls back to 30s.
func (dbc *DbClient) getStatementTimeout() time.Duration {
	if dbc.Graph == nil {
		return 30 * time.Second
	}
	cfg, err := dbc.Graph.LoadWorkspaceConfig()
	if err != nil || cfg.StatementTimeoutMs <= 0 {
		return 30 * time.Second
	}
	return time.Duration(cfg.StatementTimeoutMs) * time.Millisecond
}

// getMaxResultSizeBytes returns the workspace max_result_size_mb converted to bytes. Falls back to 100MB.
func (dbc *DbClient) getMaxResultSizeBytes() int64 {
	if dbc.Graph == nil {
		return 100 * 1024 * 1024
	}
	cfg, err := dbc.Graph.LoadWorkspaceConfig()
	if err != nil || cfg.MaxResultSizeMB <= 0 {
		return 100 * 1024 * 1024
	}
	return int64(cfg.MaxResultSizeMB) * 1024 * 1024
}

func (dbc *DbClient) getCachedMetadata(node *graph.DBInstanceNode, noCache bool) (*core.Metadata, error) {
	conn, err := dbc.getEngineConn(node)
	if err != nil {
		return nil, err
	}

	inst := engine.DBInstance{
		ID:        node.ID,
		DBType:    node.DBType,
		Proxified: node.Proxified,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return engineClient.GetMetadata(ctx, conn, inst, node.WorkspaceID, node.Name, noCache)
}

// GetMeta satisfies sqllang.MetadataFunc.
func (dbc *DbClient) GetMeta(node *graph.DBInstanceNode, noCache bool) (*core.Metadata, error) {
	return dbc.getCachedMetadata(node, noCache)
}
