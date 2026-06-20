package system

import (
	"selectDb/internal/graph"
)

// UserConfigResource describes a personal config file that lives outside the
// workspace (in the per-user config dir) but is still surfaced in the global
// resource menu and opened in the same editing experience. The frontend uses
// Kind to pick the right editor view (theme vs config) and language.
type UserConfigResource struct {
	// Name is the display/file name (e.g. ".theme", ".config").
	Name string `json:"name"`
	// URI is the selectdb://user/... URI used to read/write the file via the
	// FSProvider, just like workspace files.
	URI string `json:"uri"`
	// Kind is a stable identifier for the resource: "theme" or "config".
	Kind string `json:"kind"`
}

// GetUserConfigResources returns the per-user config files (.theme and the
// personal .config) so the frontend can list them in the global resource menu
// and open them. These resources are not part of the workspace graph; they are
// edited through their selectdb://user/... URIs.
func (s *System) GetUserConfigResources() []UserConfigResource {
	return []UserConfigResource{
		{
			Name: graph.ThemeFileName,
			URI:  graph.UserFileURI(graph.ThemeFileName),
			Kind: "theme",
		},
		{
			Name: graph.ConfigFileName,
			URI:  graph.UserFileURI(graph.ConfigFileName),
			Kind: "config",
		},
	}
}
