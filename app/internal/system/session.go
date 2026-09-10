package system

import (
	"context"
	"database/sql"
	"errors"
	"selectDb/internal/api"
	"selectDb/internal/graph"

	"selectDb/internal/desktop"
)

func (s *System) Logout() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = api.ClearAccessToken()
	_ = api.ClearRefreshToken()

	// The next person to sign in here is not necessarily the last one.
	s.closeOpenFolder()

	desktop.Emit("logout")
	return nil
}

func (s *System) closeOpenFolder() {
	graph.ClearOpenWorkspaceRoot()
	if s.Graph != nil {
		s.Graph.InvalidateWorkspaceGraph()
	}
	if s.fileWatcherCancel != nil {
		s.fileWatcherCancel()
		s.fileWatcherCancel = nil
	}
}

func (s *System) CheckForLogout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, accessErr := api.LoadAccessToken()
	_, refreshErr := api.LoadRefreshToken()

	if accessErr != nil || refreshErr != nil {
		s.closeOpenFolder()
		desktop.Emit("logout")
	}
}

func (s *System) CheckForLogin() {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, accessErr := api.LoadAccessToken()
	_, refreshErr := api.LoadRefreshToken()

	if accessErr != nil || refreshErr != nil {
		return
	}

	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := s.Queries.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = api.ClearAccessToken()
			_ = api.ClearRefreshToken()
		}
		return
	}
	desktop.Emit("login")
}
