package fs_provider

// FSProvider defines an abstract filesystem API.
//
// URIs are workspace-scoped and mirror the filesystem layout under the app
// data directory. Given an APP_ROOT, the mapping is:
//
//	selectdb://workspaces/<workspaceId>/path/inside
//
// ↔
//
//	APP_ROOT/workspaces/<workspaceId>/path/inside
//
// This ensures that URI paths and filesystem paths share the exact same
// structure once the scheme and APP_ROOT prefix are removed.
// Root is the current server folder (appRoot/<domain>/); set via SetRoot.
type FSProvider struct {
	root string
}

// New returns an FSProvider with no root; call SetRoot with the current server root after init.
func New() *FSProvider {
	return &FSProvider{}
}

// SetRoot sets the root directory (current server folder). Must be set before any FS operations.
func (fsp *FSProvider) SetRoot(root string) {
	fsp.root = root
}
