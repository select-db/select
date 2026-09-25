package datasource

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
	"sync"
	"time"

	"backend/internal/auth"
	"backend/internal/cellar"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/engine/transport"
)

var zstdDecoder, _ = zstd.NewReader(nil)

// ErrCellarUnavailable is a cellar the backend could not reach. The cause,
// which names the cellar's address, is only logged.
var ErrCellarUnavailable = errors.New("managed database temporarily unavailable, retry")

// CellarClient is the backend's side of one cellar.
type CellarClient struct {
	base   string
	tokens *cellarTokens
	http   *http.Client
}

// NewCellarClient calls the cellar at base, an http(s) URL.
func NewCellarClient(base string) *CellarClient {
	return &CellarClient{base: base, tokens: newCellarTokens(), http: &http.Client{}}
}

// Transport runs engine calls on the cellar under grant g.
func (c *CellarClient) Transport(g cellar.Grant) (*transport.HTTPTransport, error) {
	header, err := g.Encode()
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
		req.Header.Set(cellar.GrantHeader, header)
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			log.Printf("cellar: %s %s: %v", method, endpoint, err)
			return nil, ErrCellarUnavailable
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

// A token lives tokenTTL and is reused for reuseFor: each KMS sign is a remote
// call, and every reused token still has 10s left when it reaches the cellar.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

// cellarTokens signs the backend's cellar token, reusing it for reuseFor.
type cellarTokens struct {
	mu         sync.Mutex
	token      string
	reuseUntil time.Time
	sign       func(time.Duration) (string, error)
	now        func() time.Time
}

func newCellarTokens() *cellarTokens {
	return &cellarTokens{
		sign: func(ttl time.Duration) (string, error) { return auth.Sign(auth.CustomClaims{}, cellar.Audience, ttl) },
		now:  time.Now,
	}
}

// Token returns a signed token. Callers wait on one sign rather than each
// making their own.
func (t *cellarTokens) Token() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.now().Before(t.reuseUntil) {
		return t.token, nil
	}
	reuseUntil := t.now().Add(reuseFor) // taken before signing, so a slow sign only shortens reuse
	tok, err := t.sign(tokenTTL)
	if err != nil {
		return "", err
	}
	t.token, t.reuseUntil = tok, reuseUntil
	return tok, nil
}
