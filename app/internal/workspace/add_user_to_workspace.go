package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"selectDb/internal/api"
)

func (w *Workspace) AddUserToWorkspace(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("valid email is required")
	}

	ctx := context.Background()

	wtu, err := w.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no current workspace")
		}
		return fmt.Errorf("get current workspace: %w", err)
	}

	err = api.Fetch(ctx, "POST", "user/add", map[string]string{
		"workspace_id": wtu.WorkspaceID,
		"email":        email,
	}, nil, nil)
	if err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	// Pull server changes so the new user + membership appear locally
	if w.PullFunc != nil {
		_ = w.PullFunc(ctx, wtu.UserID)
	}

	return nil
}
