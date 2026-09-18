package group

import "selectDb/internal/db/generated"

// Group is the Wails-bound service for managing IAM groups (identity), their
// members, and the roles they grant. It mirrors internal/role.Role: each write
// goes through a tracked query so the SQL interceptor enqueues a sync commit.
type Group struct {
	Queries *generated.Queries
}

func New(queries *generated.Queries) *Group {
	return &Group{Queries: queries}
}
