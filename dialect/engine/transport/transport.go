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
	payload,
	response any,
) error

// FetchStreamFunc makes an authenticated request, returns raw body. Caller must close.
type FetchStreamFunc func(
	ctx context.Context,
	method,
	endpoint string,
	payload any,
) (io.ReadCloser, error)

// HTTPTransport implements engine.Transport via HTTP to the remote backend.
type HTTPTransport struct {
	Fetch       FetchFunc
	FetchStream FetchStreamFunc
}
