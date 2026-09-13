package app

// workspaceGoneHandler closes the open folder and leaves the user signed
// in. The directory and its files are untouched.
type workspaceGoneHandler struct {
	app *App
}

func (h *workspaceGoneHandler) OnWorkspaceGone() {
	if h.app.Workspace == nil {
		return
	}
	_ = h.app.Workspace.CloseFolder()
}
