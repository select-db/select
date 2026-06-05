package user

import "selectDb/backend/db/generated"

type User struct {
	Queries *generated.Queries
}

func New(Queries *generated.Queries) *User {
	return &User{
		Queries: Queries,
	}
}
