package history

import "selectDb/backend/db/generated"

type History struct {
	Queries *generated.Queries
}

func New(Queries *generated.Queries) *History {
	return &History{
		Queries: Queries,
	}
}
