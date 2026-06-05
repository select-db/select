package auth

import (
	"context"
	"selectDb/backend/db/generated"
	"selectDb/backend/db/syncer"
	"selectDb/backend/src/workspace"
)

type GithubAuth struct {
	ctx                          context.Context
	Queries                      *generated.Queries
	CancelAccessTokenPollingFunc context.CancelFunc
	WorkspaceService             *workspace.Workspace
	Syncer                       *syncer.Syncer
}

func New(Queries *generated.Queries, WorkspaceService *workspace.Workspace, Syncer *syncer.Syncer) *GithubAuth {
	return &GithubAuth{
		Queries:          Queries,
		WorkspaceService: WorkspaceService,
		Syncer:           Syncer,
	}
}

func (ga *GithubAuth) SetContext(ctx context.Context) {
	ga.ctx = ctx
}
