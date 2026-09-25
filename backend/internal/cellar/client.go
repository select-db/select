package cellar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/engine/transport"
)

var zstdDecoder, _ = zstd.NewReader(nil)

// ErrUnavailable is a cellar the backend could not reach. The cause, which
// names the cellar's address, is only logged.
var ErrUnavailable = errors.New("managed database temporarily unavailable, retry")

// Client is the backend's side of one cellar.
type Client struct {
	base   string
	tokens *Tokens
	http   *http.Client
}

// NewClient calls the cellar at base, an http(s) URL.
func NewClient(base string) *Client {
	return &Client{base: base, tokens: NewTokens(), http: &http.Client{}}
}

// Transport runs engine calls on the cellar under grant g.
func (c *Client) Transport(g Grant) (*transport.HTTPTransport, error) {
	grant, err := encodeGrant(g)
	if err != nil {
		return nil, err
	}
	token, err := c.tokens.Token()
	if err != nil {
		return nil, err
	}
	send := func(ctx context.Context, method, endpoint string, payload any, headers map[string]string) (*http.Response, error) {
		var body io.Reader
		if payload != nil {
			b, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.base+"/"+endpoint, body)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set(GrantHeader, grant)
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			log.Printf("cellar: %s %s: %v", method, endpoint, err)
			return nil, ErrUnavailable
		}
		if resp.StatusCode >= 400 {
			defer func() { _ = resp.Body.Close() }()
			msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return nil, fmt.Errorf("cellar %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
		}
		return resp, nil
	}
	return &transport.HTTPTransport{
		Fetch: func(ctx context.Context, method, endpoint string, payload any, headers map[string]string, response any) error {
			resp, err := send(ctx, method, endpoint, payload, headers)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.Header.Get("Content-Encoding") == "zstd" {
				if b, err = zstdDecoder.DecodeAll(b, nil); err != nil {
					return err
				}
			}
			if response == nil || len(b) == 0 {
				return nil
			}
			return json.Unmarshal(b, response)
		},
		FetchStream: func(ctx context.Context, method, endpoint string, payload any, headers map[string]string) (io.ReadCloser, error) {
			resp, err := send(ctx, method, endpoint, payload, headers)
			if err != nil {
				return nil, err
			}
			return resp.Body, nil
		},
	}, nil
}
