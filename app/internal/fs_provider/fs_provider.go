package fs_provider

// FSProvider defines an abstract filesystem API over selectdb:// URIs:
//
//	selectdb://workspaces/<workspaceId>/path/inside
//
// The id is looked up, since a workspace lives wherever the user opened it. The
// lookup is injected so this package and the graph resolve ids the same way.
type FSProvider struct {
	workspaceRoot func(workspaceID string) (string, error)
}

func New(workspaceRoot func(workspaceID string) (string, error)) *FSProvider {
	return &FSProvider{workspaceRoot: workspaceRoot}
}
