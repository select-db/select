package system

import (
	"context"
	"database/sql"
	"errors"
	"selectDb/internal/api"

	"selectDb/internal/desktop"
)

func (s *System) Logout() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = api.ClearAccessToken()
	_ = api.ClearRefreshToken()

	desktop.Emit("logout")
	return nil
}

func (s *System) CheckForLogout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, accessErr := api.LoadAccessToken()
	_, refreshErr := api.LoadRefreshToken()

	if accessErr != nil || refreshErr != nil {
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
