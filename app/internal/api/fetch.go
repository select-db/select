package api

import (
	"bytes"
	"context"
	"encoding/json"
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

	refreshMu     sync.Mutex
	refreshedAt   time.Time
	refreshFailed bool

	// In-process fallback for the rotated refresh token. The backend rotates and
	// deletes the old refresh token on every refresh, so if the keyring write of the
	// new token fails the session would be stranded on a dead token and 401 forever.
	// Holding the latest token here keeps the running session alive across keyring
	// flakiness; it is cleared once a keyring write succeeds.
	memTokenMu      sync.RWMutex
	memRefreshToken string
)

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
func doWithRetry(
	ctx context.Context,
	client *http.Client,
	method,
	endpoint string,
	payload interface{},
	headers map[string]string,
) (*http.Response, error) {
	resp, err := doRequest(
		ctx,
		client,
		method,
		endpoint,
		payload,
		headers,
		false,
	)
	if err != nil {
		return nil, err
	}

	saveTokensFromResponse(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		// Request succeeded with the current access token; clear any prior
		// refresh-failure latch. Done under the lock since refreshFailed is also
		// written by the refresh path below.
		refreshMu.Lock()
		refreshFailed = false
		refreshMu.Unlock()
		return resp, nil
	}
	_ = resp.Body.Close()

	beforeRefresh := time.Now()

	refreshMu.Lock()
	if refreshedAt.After(beforeRefresh) {
		// Another goroutine already refreshed while we waited. Capture the latch
		// before unlocking so the read stays synchronized with the refresh path.
		failed := refreshFailed
		refreshMu.Unlock()
		if failed {
			return nil, fmt.Errorf("session expired, please log in again")
		}
		return doRequest(ctx, client, method, endpoint, payload, headers, false)
	}

	resp, err = doRequest(ctx, client, method, endpoint, payload, headers, true)
	if err != nil {
		refreshFailed = true
		refreshedAt = time.Now()
		refreshMu.Unlock()
		return nil, err
	}

	saveTokensFromResponse(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		_ = clearAccessTokenFunc()
		_ = clearRefreshTokenFunc()
		refreshFailed = true
		refreshedAt = time.Now()
		refreshMu.Unlock()
		return nil, fmt.Errorf("session expired, please log in again")
	}

	refreshFailed = false
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
	includeRefresh bool,
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

	if accessToken, err := loadAccessTokenFunc(); err == nil {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	if includeRefresh {
		if refreshToken, err := currentRefreshToken(); err == nil {
			req.Header.Set("X-Refresh-Token", refreshToken)
		}
		if deviceID, err := loadDeviceIDFunc(); err == nil {
			req.Header.Set("X-Device-ID", deviceID)
		}
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
