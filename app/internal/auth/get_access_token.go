package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"selectDb/internal/api"
	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
	"selectDb/internal/workspace"

	"selectDb/internal/desktop"
)

type GetAccessTokenParams struct {
	DeviceCode string `json:"device_code"`
	DeviceID   string `json:"device_id"`
}

type BackendUserResponse struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	AvatarURL string `json:"AvatarURL"`
	IsNew     bool   `json:"IsNew"`
}

// accessTokenPoll is the server's get-access-token reply: status "pending"
// (re-poll after Interval s) or "complete" (tokens already saved from headers).
type accessTokenPoll struct {
	Status    string `json:"status"`
	Interval  int    `json:"interval"`
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	AvatarURL string `json:"AvatarURL"`
}

func (ga *GithubAuth) GetAccessToken(deviceCode string) error {
	if deviceCode == "" {
		return fmt.Errorf("deviceCode is required")
	}

	deviceID, err := api.LoadDeviceID()
	if err != nil {
		return fmt.Errorf("failed to load device ID: %w", err)
	}

	if len(deviceID) == 0 {
		return fmt.Errorf("device ID is required but was empty")
	}

	ga.CancelAccessTokenPolling()

	// 90s overall budget; the server does one GitHub round-trip per call and
	// tells us when to poll again, so the loop lives here, not on the server.
	logUserCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	ga.CancelAccessTokenPollingFunc = cancel
	defer cancel()

	params := GetAccessTokenParams{DeviceCode: deviceCode, DeviceID: deviceID}
	var backendUser BackendUserResponse
	for {
		var pr accessTokenPoll
		if ferr := api.Fetch(logUserCtx, "POST", "auth/access-token", params, nil, &pr); ferr != nil {
			return ferr
		}
		if pr.Status == "complete" {
			backendUser = BackendUserResponse{ID: pr.ID, Name: pr.Name, AvatarURL: pr.AvatarURL}
			break
		}
		wait := time.Duration(pr.Interval) * time.Second
		if wait < 5*time.Second {
			wait = 5 * time.Second
		}
		select {
		case <-logUserCtx.Done():
			return fmt.Errorf("authorization timed out")
		case <-time.After(wait):
		}
	}

	ctx := context.Background()

	user, err := ga.Queries.UpsertUser(ctx, generated.UpsertUserParams{
		ID:   backendUser.ID,
		Name: db_types.NewJSONNullString(backendUser.Name),
	})
	if err != nil {
		return err
	}

	// Pull changes and push any pending commits
	// same path for login, deco/reco, new commit.
	if err := ga.Syncer.Sync(ctx, user.ID); err != nil {
		return fmt.Errorf("sync after login: %w", err)
	}

	_, err = ga.Queries.GetCurrentWorkspace(ctx, user.ID)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			_, err := ga.WorkspaceService.SetOrCreateCurrentWorkspace(
				workspace.SetOrCreateCurrentWorkspaceParams{
					UserID: user.ID,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to set or create current workspace: %w", err)
			}
		default:
			return fmt.Errorf("failed to get current workspace: %w", err)
		}
	}

	_ = utils.FetchAndSaveAvatar(ctx, backendUser.AvatarURL, user.ID)

	desktop.Emit("login")
	return nil
}

func (ga *GithubAuth) CancelAccessTokenPolling() {
	if ga.CancelAccessTokenPollingFunc != nil {
		ga.CancelAccessTokenPollingFunc()
	}
}
