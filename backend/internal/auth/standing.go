package auth

// The caller's standing: who they are to a workspace. It is request state,
// derived per request for a signed-in user and read off the key row for an API
// key, and it is deliberately not in the access token. A role taken away has to
// stop granting on the next request rather than when a token expires.

// RoleRef names a role the caller holds. The name rides along so audit and the
// UI can label one without a lookup.
type RoleRef struct {
	ID   string
	Name string
}

// WorkspaceStanding is what the caller is to one workspace: a member, possibly
// its owner, holding some roles in it.
type WorkspaceStanding struct {
	ID      string
	IsOwner bool
	Roles   []RoleRef
}
