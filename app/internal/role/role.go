package role

import "selectDb/internal/db/generated"

type Role struct {
	Queries *generated.Queries
}

func New(queries *generated.Queries) *Role {
	return &Role{Queries: queries}
}
