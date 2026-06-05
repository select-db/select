package datasource

import (
	"context"
	"fmt"

	"selectDb/backend/api"
	"selectDb/backend/db/generated"
)

type Datasource struct {
	Queries *generated.Queries
}

func New(queries *generated.Queries) *Datasource {
	return &Datasource{Queries: queries}
}

func (d *Datasource) currentWorkspaceID(ctx context.Context) (string, error) {
	user, err := d.Queries.GetCurrentUser(ctx)
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}
	ws, err := d.Queries.GetCurrentWorkspace(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("get current workspace: %w", err)
	}
	return ws.ID, nil
}

type GetResult struct {
	Name            string `json:"name"`
	DSN             string `json:"dsn"`
	SSH             string `json:"ssh"`
	MaxOpenConns    int64  `json:"max_open_conns"`
	MaxIdleConns    int64  `json:"max_idle_conns"`
	ConnMaxLifetime int64  `json:"conn_max_lifetime"`
	ConnMaxIdleTime int64  `json:"conn_max_idle_time"`
}

func (d *Datasource) GetDatasource(id string) (*GetResult, error) {
	ctx := context.Background()
	workspaceID, err := d.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	var result GetResult
	if err := api.Fetch(ctx, "POST", "datasource/get", map[string]string{
		"id":           id,
		"workspace_id": workspaceID,
	}, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type UpsertParams struct {
	ID              string `json:"id"`
	DBType          string `json:"db_type"`
	Name            string `json:"name"`
	DSN             string `json:"dsn"`
	SSH             string `json:"ssh"` // JSON-encoded SSH config
	MaxOpenConns    int64  `json:"max_open_conns"`
	MaxIdleConns    int64  `json:"max_idle_conns"`
	ConnMaxLifetime int64  `json:"conn_max_lifetime"`
	ConnMaxIdleTime int64  `json:"conn_max_idle_time"`
}

type upsertRemoteParams struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	DBType          string `json:"db_type"`
	Name            string `json:"name"`
	DSN             string `json:"dsn"`
	SSH             string `json:"ssh"`
	MaxOpenConns    int64  `json:"max_open_conns"`
	MaxIdleConns    int64  `json:"max_idle_conns"`
	ConnMaxLifetime int64  `json:"conn_max_lifetime"`
	ConnMaxIdleTime int64  `json:"conn_max_idle_time"`
}

func (d *Datasource) UpsertDatasource(params UpsertParams) error {
	ctx := context.Background()
	workspaceID, err := d.currentWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return api.Fetch(ctx, "POST", "datasource/upsert", upsertRemoteParams{
		ID:              params.ID,
		WorkspaceID:     workspaceID,
		DBType:          params.DBType,
		Name:            params.Name,
		DSN:             params.DSN,
		SSH:             params.SSH,
		MaxOpenConns:    params.MaxOpenConns,
		MaxIdleConns:    params.MaxIdleConns,
		ConnMaxLifetime: params.ConnMaxLifetime,
		ConnMaxIdleTime: params.ConnMaxIdleTime,
	}, nil, nil)
}

type deleteRemoteParams struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
}

func (d *Datasource) DeleteDatasource(id string) error {
	ctx := context.Background()
	workspaceID, err := d.currentWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return api.Fetch(ctx, "POST", "datasource/delete", deleteRemoteParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}, nil, nil)
}
