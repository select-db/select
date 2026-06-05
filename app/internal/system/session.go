package system

import (
	"context"
	"database/sql"
	"errors"
	"selectDb/internal/api"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (s *System) Logout() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = api.ClearAccessToken()
	_ = api.ClearRefreshToken()

	runtime.EventsEmit(s.ctx, "logout")
	return nil
}

func (s *System) CheckForLogout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, accessErr := api.LoadAccessToken()
	_, refreshErr := api.LoadRefreshToken()

	if accessErr != nil || refreshErr != nil {
		runtime.EventsEmit(s.ctx, "logout")
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
	runtime.EventsEmit(s.ctx, "login")
}
