package cellarclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/toolkit/cache"
)

var httpClient = &http.Client{}

// The service token lives tokenTTL and is reused for the 50s window it was
// signed in, so it has 10s left when it reaches the cellar. Each sign is a KMS call.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

var tokens = cache.New(cache.Options{MaxEntries: 1})

// request sends one signed request to the cellar. A cellar it cannot reach is
// ErrUnavailable; the cause, which names the cellar's address, is only logged.
func request(ctx context.Context, method, path, grant string, body []byte) (*http.Response, error) {
	if URL == "" {
		return nil, ErrOff
	}
	window := strconv.FormatInt(time.Now().Unix()/int64(reuseFor.Seconds()), 10)
	token, err := tokens.GetOrCreate(window, func() (any, error) {
		return auth.Sign(auth.CustomClaims{}, cellar.Audience, tokenTTL)
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.(string))
	req.Header.Set(cellar.GrantHeader, grant)
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("cellar: %s %s: %v", method, path, err)
		return nil, ErrUnavailable
	}
	return resp, nil
}

// callCellar sends a request and returns the response only when the cellar
// succeeded; a failure comes back as the cellar's coded error.
func callCellar(ctx context.Context, method, path, grant string, body []byte) (*http.Response, error) {
	resp, err := request(ctx, method, path, grant, body)
	if err != nil || resp.StatusCode < 300 {
		return resp, err
	}
	defer func() { _ = resp.Body.Close() }()
	errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var coded arrowstream.Error
	if json.Unmarshal(errorBody, &coded) == nil && coded.Code != "" {
		return nil, &coded
	}
	return nil, cellar.InternalError(fmt.Sprintf("cellar: %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(errorBody))))
}
