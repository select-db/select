package auth

import (
	"context"
	"selectDb/internal/db/generated"
	"selectDb/internal/db/syncer"
)

type GithubAuth struct {
	Queries                      *generated.Queries
	CancelAccessTokenPollingFunc context.CancelFunc
	Syncer                       *syncer.Syncer
}

func New(Queries *generated.Queries, Syncer *syncer.Syncer) *GithubAuth {
	return &GithubAuth{
		Queries: Queries,
		Syncer:  Syncer,
	}
}
