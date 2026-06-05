package workspace

import (
	"context"
	"fmt"
	"strings"

	"selectDb/backend/api"
)

type SearchUserResult struct {
	Found         bool    `json:"found"`
	UserID        string  `json:"user_id,omitempty"`
	Name          *string `json:"name,omitempty"`
	Email         string  `json:"email"`
	AlreadyAdded bool    `json:"already_added"`
}

func (w *Workspace) SearchUser(email string) (*SearchUserResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	ctx := context.Background()

	wtu, err := w.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current workspace: %w", err)
	}

	var result SearchUserResult
	err = api.Fetch(ctx, "POST", "user/search", map[string]string{
		"workspace_id": wtu.WorkspaceID,
		"email":        email,
	}, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("search user: %w", err)
	}

	return &result, nil
}
