package datasource

import (
	"context"
	"fmt"

	"selectDb/internal/api"
	"selectDb/internal/db/generated"
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
	if err := api.Fetch(ctx, "GET", "datasources/"+id, nil, api.WorkspaceHeader(workspaceID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListedDatasource is one proxified connection as the connections screen shows
// it. No DSN: administrating a connection does not require being handed the
// credential behind it.
type ListedDatasource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	DBType string `json:"db_type"`
}

// ListDatasources returns every proxified connection stored for the workspace,
// including ones no workspace file names any more.
//
// That last case is the reason this exists. The directory naming a connection
// is replicated through git, so it can be deleted on another machine, in a
// branch, or outside the app, while the credential stays on the server. Without
// this list such a connection cannot be seen or revoked, because the id needed
// to name it lived in the file that was deleted.
func (d *Datasource) ListDatasources() ([]ListedDatasource, error) {
	ctx := context.Background()
	workspaceID, err := d.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	var result []ListedDatasource
	if err := api.Fetch(ctx, "GET", "datasources", nil, api.WorkspaceHeader(workspaceID), &result); err != nil {
		return nil, err
	}
	return result, nil
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
	return api.Fetch(ctx, "PUT", "datasources/"+params.ID, upsertRemoteParams{
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
	}, api.WorkspaceHeader(workspaceID), nil)
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
	return api.Fetch(ctx, "DELETE", "datasources/"+id, deleteRemoteParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}, api.WorkspaceHeader(workspaceID), nil)
}
