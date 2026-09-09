package app

// currentWorkspaceGoneHandler closes the open folder and leaves the user signed
// in. The directory and its files are untouched.
type currentWorkspaceGoneHandler struct {
	app *App
}

func (h *currentWorkspaceGoneHandler) OnCurrentWorkspaceGone() {
	if h.app.Workspace == nil {
		return
	}
	_ = h.app.Workspace.CloseFolder()
}
