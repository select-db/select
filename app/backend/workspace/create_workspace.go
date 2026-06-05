package workspace

import (
	"context"
	"fmt"
	"selectDb/backend/db"
	"selectDb/backend/db/db_types"
	"selectDb/backend/db/generated"
)

type CreateWorkspaceParams struct {
	ID                string
	WorkspaceToUserID string
	UserID            string
	Name              string
}

func (w *Workspace) CreateWorkspace(params CreateWorkspaceParams) (generated.Workspace, error) {
	if params.ID == "" || params.WorkspaceToUserID == "" || params.UserID == "" {
		return generated.Workspace{}, fmt.Errorf("ID, WorkspaceToUserID, and UserID are required")
	}

	ctx := context.Background()

	DB := db.GetDB()
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return generated.Workspace{}, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := w.Queries.WithTx(tx)

	name := params.Name
	if name == "" {
		name = "My workspace"
	}

	workspace, err := qtx.CreateWorkspace(ctx, generated.CreateWorkspaceParams{
		ID:      params.ID,
		Name:    name,
		OwnerID: db_types.NewJSONNullStringFromPtr(&params.UserID),
	})
	if err != nil {
		return generated.Workspace{}, err
	}

	if err := qtx.ClearCurrentWorkspaceToUser(ctx); err != nil {
		return generated.Workspace{}, err
	}

	if _, err = qtx.CreateWorkspaceToUser(ctx, generated.CreateWorkspaceToUserParams{
		ID:          params.WorkspaceToUserID,
		UserID:      params.UserID,
		WorkspaceID: workspace.ID,
	}); err != nil {
		return generated.Workspace{}, err
	}

	if err := qtx.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      params.UserID,
		WorkspaceID: workspace.ID,
	}); err != nil {
		return generated.Workspace{}, err
	}

	if err := tx.Commit(); err != nil {
		return generated.Workspace{}, err
	}

	return workspace, nil
}
