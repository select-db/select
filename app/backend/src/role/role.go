package role

import "selectDb/backend/db/generated"

type Role struct {
	Queries *generated.Queries
}

func New(queries *generated.Queries) *Role {
	return &Role{Queries: queries}
}
