package apikey

import (
	"context"
	"fmt"

	"selectDb/internal/api"
	"selectDb/internal/db/generated"
)

type APIKey struct {
	Queries *generated.Queries
}

func New(queries *generated.Queries) *APIKey {
	return &APIKey{Queries: queries}
}

func (a *APIKey) currentWorkspaceID(ctx context.Context) (string, error) {
	user, err := a.Queries.GetCurrentUser(ctx)
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}
	ws, err := a.Queries.GetCurrentWorkspace(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("get current workspace: %w", err)
	}
	return ws.ID, nil
}

type APIKeyRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIKeyEntry struct {
	ID         string       `json:"id"`
	Name       *string      `json:"name"`
	Prefix     *string      `json:"prefix"`
	Roles      []APIKeyRole `json:"roles"`
	CreatedBy  *string      `json:"created_by"`
	CreatedAt  *string      `json:"created_at"`
	LastUsedAt *string      `json:"last_used_at"`
	ExpiresAt  *string      `json:"expires_at"`
}

type CreateParams struct {
	Name      string   `json:"name"`
	RoleIDs   []string `json:"role_ids"`
	ExpiresAt *string  `json:"expires_at"` // RFC3339, nil = never expires
}

// CreateResult carries the plaintext key. It is returned once and is never
// retrievable again.
type CreateResult struct {
	ID     string `json:"id"`
	Prefix string `json:"prefix"`
	Key    string `json:"key"`
}

func (a *APIKey) ListAPIKeys() ([]APIKeyEntry, error) {
	ctx := context.Background()
	workspaceID, err := a.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	var result []APIKeyEntry
	if err := api.Fetch(ctx, "POST", "apikey/list", map[string]string{
		"workspace_id": workspaceID,
	}, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *APIKey) CreateAPIKey(params CreateParams) (*CreateResult, error) {
	ctx := context.Background()
	workspaceID, err := a.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"workspace_id": workspaceID,
		"name":         params.Name,
		"role_ids":     params.RoleIDs,
		"expires_at":   params.ExpiresAt,
	}
	var result CreateResult
	if err := api.Fetch(ctx, "POST", "apikey/create", body, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *APIKey) RevokeAPIKey(id string) error {
	ctx := context.Background()
	workspaceID, err := a.currentWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return api.Fetch(ctx, "POST", "apikey/revoke", map[string]string{
		"workspace_id": workspaceID,
		"id":           id,
	}, nil, nil)
}

func (a *APIKey) SetAPIKeyRoles(id string, roleIDs []string) error {
	ctx := context.Background()
	workspaceID, err := a.currentWorkspaceID(ctx)
	if err != nil {
		return err
	}
	if roleIDs == nil {
		roleIDs = []string{}
	}
	return api.Fetch(ctx, "POST", "apikey/set-roles", map[string]interface{}{
		"workspace_id": workspaceID,
		"id":           id,
		"role_ids":     roleIDs,
	}, nil, nil)
}

func (a *APIKey) RotateAPIKey(id string) (*CreateResult, error) {
	ctx := context.Background()
	workspaceID, err := a.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	var result CreateResult
	if err := api.Fetch(ctx, "POST", "apikey/rotate", map[string]string{
		"workspace_id": workspaceID,
		"id":           id,
	}, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
