// Package transport implements engine.Transport via HTTP to the remote backend.
package transport

import (
	"context"
	"io"
)

// FetchFunc abstracts an authenticated HTTP call. App injects api.Fetch.
type FetchFunc func(
	ctx context.Context,
	method,
	endpoint string,
	payload any,
	headers map[string]string,
	response any,
) error

// FetchStreamFunc makes an authenticated request, returns raw body. Caller must close.
type FetchStreamFunc func(
	ctx context.Context,
	method,
	endpoint string,
	payload any,
	headers map[string]string,
) (io.ReadCloser, error)

// headerWorkspaceID selects the target workspace for workspace-scoped endpoints.
const headerWorkspaceID = "X-Workspace-Id"

// workspaceHeader scopes a request to a workspace via the X-Workspace-Id header.
func workspaceHeader(workspaceID string) map[string]string {
	return map[string]string{headerWorkspaceID: workspaceID}
}

// HTTPTransport implements engine.Transport via HTTP to the remote backend.
type HTTPTransport struct {
	Fetch       FetchFunc
	FetchStream FetchStreamFunc
}
