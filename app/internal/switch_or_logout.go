package backend

import (
	"time"

	"selectDb/internal/utils"
)

// switchOrLogoutHandler implements syncer.SwitchOrLogoutHandler so the syncer can emit and logout after applying server deletes.
type switchOrLogoutHandler struct {
	app *App
}

func (h *switchOrLogoutHandler) OnAfterWorkspaceSwitch() {
	if h.app.ctx != nil && h.app.Graph != nil && h.app.Graph.WorkspaceGraph != nil {
		utils.DebouncedEventsEmit(h.app.ctx, "workspaceGraphUpdated", 100*time.Millisecond, h.app.Graph.WorkspaceGraph)
	}
}

func (h *switchOrLogoutHandler) OnLogout() {
	// Invalidate the in-memory graph before logging out.
	if h.app.Graph != nil {
		h.app.Graph.InvalidateWorkspaceGraph()
	}
	if h.app.System != nil {
		_ = h.app.System.Logout()
	}
}
