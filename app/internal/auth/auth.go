package auth

import (
	"context"
	"selectDb/internal/db/generated"
	"selectDb/internal/db/syncer"
	"selectDb/internal/workspace"
)

type GithubAuth struct {
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
