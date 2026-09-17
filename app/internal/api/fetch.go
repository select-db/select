package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"selectDb/internal/server"

	"github.com/klauspost/compress/zstd"
)

var zstdDecoder, _ = zstd.NewReader(nil)

// HeaderWorkspaceID selects the target workspace for workspace-scoped endpoints.
const HeaderWorkspaceID = "X-Workspace-Id"

// WorkspaceHeader builds the header map that scopes a request to a workspace.
func WorkspaceHeader(workspaceID string) map[string]string {
	return map[string]string{HeaderWorkspaceID: workspaceID}
}

var (
	httpClient = &http.Client{
		Timeout: 3 * time.Minute,
	}

	// Longer ceiling so the HTTP layer doesn't cut streaming queries early
	httpStreamClient = &http.Client{
		Timeout: 45 * time.Minute,
	}

	loadAccessTokenFunc  = LoadAccessToken
	saveAccessTokenFunc  = SaveAccessToken
	clearAccessTokenFunc = ClearAccessToken

	loadRefreshTokenFunc  = LoadRefreshToken
	saveRefreshTokenFunc  = SaveRefreshToken
	clearRefreshTokenFunc = ClearRefreshToken

	loadDeviceIDFunc = LoadDeviceID

	refreshMu   sync.Mutex
	refreshedAt time.Time

	// Why the last refresh failed, or nil. Requests that queued behind it are
	// handed this rather than a guess, so a network blip is never reported as a
	// session that ended.
	refreshErr error

	// In-process fallback for the rotated refresh token. The backend rotates and
	// deletes the old refresh token on every refresh, so if the keyring write of the
	// new token fails the session would be stranded on a dead token and 401 forever.
	// Holding the latest token here keeps the running session alive across keyring
	// flakiness; it is cleared once a keyring write succeeds.
	memTokenMu      sync.RWMutex
	memRefreshToken string
)

// errSessionExpired is returned only once the server has been handed a complete
// refresh credential set and has rejected it. Every caller that surfaces a
// logout keys off this.
var errSessionExpired = errors.New("session expired, please log in again")

// refreshCreds is everything the server needs to run its refresh path: the
// expired access token names the session, the rotated refresh token and the
// device ID authorize the new pair. Missing any one of them, the server answers
// 401 without ever looking at the session.
type refreshCreds struct {
	accessToken  string
	refreshToken string
	deviceID     string
}

// loadRefreshCreds reports whether a refresh is worth attempting at all. A
// keyring that cannot be read is not an expired session, so the caller keeps the
// server's 401 instead of destroying credentials it never managed to present.
func loadRefreshCreds() (refreshCreds, bool) {
	accessToken, err := loadAccessTokenFunc()
	if err != nil || accessToken == "" {
		return refreshCreds{}, false
	}
	refreshToken, err := currentRefreshToken()
	if err != nil || refreshToken == "" {
		return refreshCreds{}, false
	}
	deviceID, err := loadDeviceIDFunc()
	if err != nil || deviceID == "" {
		return refreshCreds{}, false
	}
	return refreshCreds{accessToken: accessToken, refreshToken: refreshToken, deviceID: deviceID}, true
}

// rememberRefreshToken persists the rotated refresh token to the keyring, falling
// back to in-process memory when the keyring write fails so the session survives.
func rememberRefreshToken(token string) {
	if err := saveRefreshTokenFunc(token); err != nil {
		memTokenMu.Lock()
		memRefreshToken = token
		memTokenMu.Unlock()
		return
	}
	memTokenMu.Lock()
	memRefreshToken = ""
	memTokenMu.Unlock()
}

// forgetRefreshToken drops the in-process copy, so a token never outlives the
// session it belongs to and is never presented on the next one.
func forgetRefreshToken() {
	memTokenMu.Lock()
	memRefreshToken = ""
	memTokenMu.Unlock()
}

// currentRefreshToken returns the freshest refresh token: the in-process value when a
// prior keyring write failed, otherwise the keyring value.
func currentRefreshToken() (string, error) {
	memTokenMu.RLock()
	mem := memRefreshToken
	memTokenMu.RUnlock()
	if mem != "" {
		return mem, nil
	}
	return loadRefreshTokenFunc()
}

func Fetch(
	ctx context.Context,
	method,
	endpoint string,
	payload interface{},
	headers map[string]string,
	response interface{},
) error {
	resp, err := doWithRetry(
		ctx,
		httpClient,
		method,
		endpoint,
		payload,
		headers,
	)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.Header.Get("Content-Encoding") == "zstd" {
		respBodyBytes, err = zstdDecoder.DecodeAll(respBodyBytes, nil)
		if err != nil {
			return fmt.Errorf("failed to decompress zstd response: %w", err)
		}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBodyBytes))
	}

	if response != nil && len(respBodyBytes) > 0 {
		if err := json.Unmarshal(respBodyBytes, response); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	return nil
}

// Caller must close the returned body.
func FetchStream(
	ctx context.Context,
	method,
	endpoint string,
	payload interface{},
	headers map[string]string,
) (io.ReadCloser, error) {
	resp, err := doWithRetry(
		ctx,
		httpStreamClient,
		method,
		endpoint,
		payload,
		headers,
	)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func GetBaseURL() (string, error) {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return "", err
	}
	if domain != "" {
		return server.DomainToBaseURL(domain), nil
	}
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		return "", fmt.Errorf("no server selected and API_URL is not set")
	}
	return apiURL, nil
}

// Retries on 401 with token refresh. Concurrent 401s are serialized:
// first goroutine refreshes, others wait then retry.
//
// A 401 only ends the session when the server was handed a complete refresh
// credential set and still refused it. A 401 we could attach no credentials to,
// one the endpoint itself returned after the server had refreshed, and a refresh
// that never reached the server all leave the stored credentials alone: treating
// them as proof of expiry signed people out of live sessions.
func doWithRetry(
	ctx context.Context,
	client *http.Client,
	method,
	endpoint string,
	payload interface{},
	headers map[string]string,
) (*http.Response, error) {
	resp, err := doRequest(ctx, client, method, endpoint, payload, headers, nil)
	if err != nil {
		return nil, err
	}

	saveTokensFromResponse(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		// Request succeeded with the current access token; clear any prior
		// refresh failure. Done under the lock since refreshErr is also written by
		// the refresh path below.
		refreshMu.Lock()
		refreshErr = nil
		refreshMu.Unlock()
		return resp, nil
	}

	creds, ok := loadRefreshCreds()
	if !ok {
		// Nothing to refresh with, so this 401 is the server's answer to the
		// request, not a verdict on the session.
		return resp, nil
	}
	_ = resp.Body.Close()

	beforeRefresh := time.Now()

	refreshMu.Lock()
	if refreshedAt.After(beforeRefresh) {
		// Another goroutine already refreshed while we waited. Capture the outcome
		// before unlocking so the read stays synchronized with the refresh path.
		failure := refreshErr
		refreshMu.Unlock()
		if failure != nil {
			return nil, failure
		}
		return doRequest(ctx, client, method, endpoint, payload, headers, nil)
	}

	resp, err = doRequest(ctx, client, method, endpoint, payload, headers, &creds)
	if err != nil {
		// The server never answered, so nothing is known about the refresh token.
		// Waiters get this error rather than a logout.
		refreshErr = err
		refreshedAt = time.Now()
		refreshMu.Unlock()
		return nil, err
	}

	saveTokensFromResponse(resp)

	// A new access token means the server ran its refresh path, so a 401 behind it
	// belongs to the endpoint and says nothing about the session.
	if resp.StatusCode == http.StatusUnauthorized && resp.Header.Get("X-New-Access-Token") == "" {
		_ = resp.Body.Close()
		_ = clearAccessTokenFunc()
		_ = clearRefreshTokenFunc()
		forgetRefreshToken()
		refreshErr = errSessionExpired
		refreshedAt = time.Now()
		refreshMu.Unlock()
		return nil, errSessionExpired
	}

	refreshErr = nil
	refreshedAt = time.Now()
	refreshMu.Unlock()
	return resp, nil
}

func doRequest(
	ctx context.Context,
	client *http.Client,
	method,
	endpoint string,
	payload interface{},
	headers map[string]string,
	refresh *refreshCreds,
) (*http.Response, error) {
	apiURL, err := GetBaseURL()
	if err != nil {
		return nil, err
	}

	var bodyBytes []byte
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		apiURL+"/"+endpoint,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if refresh != nil {
		req.Header.Set("Authorization", "Bearer "+refresh.accessToken)
		req.Header.Set("X-Refresh-Token", refresh.refreshToken)
		req.Header.Set("X-Device-ID", refresh.deviceID)
	} else if accessToken, err := loadAccessTokenFunc(); err == nil {
		// Best effort: the login endpoints are reached before there is one.
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func saveTokensFromResponse(resp *http.Response) {
	if newAccess := resp.Header.Get("X-New-Access-Token"); newAccess != "" {
		_ = saveAccessTokenFunc(newAccess)
	}
	if newRefresh := resp.Header.Get("X-New-Refresh-Token"); newRefresh != "" {
		rememberRefreshToken(newRefresh)
	}
}
