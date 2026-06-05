package db

import (
	"selectDb/backend/db/generated"
	"selectDb/backend/db/syncer"

	"github.com/ngrok/sqlmw"
)

type InternalDb struct {
	Queries *generated.Queries
	Syncer  *syncer.Syncer
	sqlmw.NullInterceptor
}

func New(Queries *generated.Queries, Syncer *syncer.Syncer) *InternalDb {
	return &InternalDb{
		Queries: Queries,
		Syncer:  Syncer,
	}
}
