package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/utils"
)

type PollTokenRequest struct {
	DeviceCode string `json:"device_code"`
	DeviceID   string `json:"device_id"`
}

type GitHubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}

type AccessTokenResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	Scope       string      `json:"scope"`
	User        *GitHubUser `json:"user,omitempty"`
	Error       string      `json:"error,omitempty"`
	ErrorDesc   string      `json:"error_description,omitempty"`
	Interval    int         `json:"interval,omitempty"`
}

// pollSem caps concurrent in-flight GitHub exchanges. With one short GitHub
// round-trip per request (the client owns the retry loop) the hold is ~ms, so
// this is purely an abuse bound, not a cap on legitimate concurrent logins.
var pollSem = make(chan struct{}, 256)

var (
	errAuthDenied  = errors.New("authorization was denied")
	errCodeExpired = errors.New("device code expired")
)

// GitHub endpoints, as vars so tests can point the exchange at a mock server.
var (
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	githubUserURL       = "https://api.github.com/user"
	githubUserEmailsURL = "https://api.github.com/user/emails"
)

// signInRejectedError is a terminal failure where GitHub proved identity but the
// app refused the sign-in (email not on the allowlist, or no verified email). It
// carries the attempted email so the handler can record it on auth.login_failed.
type signInRejectedError struct {
	email  string
	reason string
}

func (e *signInRejectedError) Error() string { return "sign-in not allowed: " + e.reason }

// pollResponse is the get-access-token body. Status "pending" => the client
// re-polls after Interval seconds; "complete" => tokens are in the X-New-*
// headers and the user fields are populated.
type pollResponse struct {
	Status   string `json:"status"`
	Interval int    `json:"interval,omitempty"`
	*LoggedUser
}

func GetAccessTokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PollTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.DeviceCode == "" || req.DeviceID == "" {
			http.Error(w, "Missing device_code or device_id", http.StatusBadRequest)
			return
		}

		// Reject codes we didn't issue (else open proxy to GitHub)
		if !deviceCodeKnown(req.DeviceCode) {
			http.Error(w, "unknown or expired device code", http.StatusBadRequest)
			return
		}

		select {
		case pollSem <- struct{}{}:
			defer func() { <-pollSem }()
		default:
			http.Error(w, "server busy, retry shortly", http.StatusServiceUnavailable)
			return
		}

		// One GitHub round-trip; the desktop client owns the overall budget.
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		user, retryAfter, err := exchangeDeviceCode(ctx, req.DeviceCode)
		if err != nil {
			var rejected *signInRejectedError
			switch {
			case errors.Is(err, errAuthDenied):
				emitLoginFailed(ctx, r, "", "access_denied")
				http.Error(w, "authorization was denied", http.StatusForbidden)
			case errors.Is(err, errCodeExpired):
				http.Error(w, "device code expired", http.StatusGone)
			case errors.As(err, &rejected):
				// GitHub proved identity but the app refused the sign-in: a
				// security-relevant failure, not a system fault.
				emitLoginFailed(ctx, r, rejected.email, rejected.reason)
				http.Error(w, "sign-in not allowed for this account", http.StatusForbidden)
			default:
				log.Printf("auth: device exchange: %v", err)
				http.Error(w, "authorization failed", http.StatusBadGateway)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if user == nil {
			// Not authorised yet, tell the client to keep polling.
			_ = json.NewEncoder(w).Encode(pollResponse{Status: "pending", Interval: retryAfter})
			return
		}

		accessToken, err := CreateJWT(ctx, user.ID)
		if err != nil {
			log.Printf("auth: create jwt: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		issuedIP := GetIPAddress(r)
		refreshToken, err := CreateRefreshToken(context.Background(), user.ID, req.DeviceID, issuedIP)
		if err != nil {
			log.Printf("auth: create refresh token: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("X-New-Access-Token", accessToken)
		w.Header().Set("X-New-Refresh-Token", *refreshToken)

		// Login is the user's own security event: attributed to them, no workspace
		// (an org admin never sees a member's personal login), like token refresh.
		emitLogin(ctx, r, user)

		if err := json.NewEncoder(w).Encode(pollResponse{Status: "complete", LoggedUser: user}); err != nil {
			http.Error(w, "Failed to encode user response", http.StatusInternalServerError)
			return
		}
	}
}

type LoggedUser struct {
	ID        db_types.JSONNullUUID
	Name      db_types.JSONNullString
	AvatarURL string
}

// emitLogin records a successful authentication as the user's own auth event
// (no workspace). Best-effort: a logging failure never fails the login.
func emitLogin(ctx context.Context, r *http.Request, user *LoggedUser) {
	if err := audit.Emit(ctx, audit.AuthLogin, audit.Record{
		Principal: audit.Principal{Type: audit.PrincipalUser, ID: user.ID.String(), Name: user.Name.ValueOrEmpty()},
		Status:    audit.StatusSuccess,
		ClientIP:  GetIPAddress(r),
	}); err != nil {
		log.Printf("audit: emit %s: %v", audit.AuthLogin.Type(), err)
	}
}

// emitLoginFailed records a rejected sign-in. The principal is unknown (the app
// user may not exist), so the attempted email/reason go in the payload; a SOC
// reads these as brute-force / unauthorized-access signals.
func emitLoginFailed(ctx context.Context, r *http.Request, email, reason string) {
	payload := map[string]any{"provider": "github", "reason": reason}
	if email != "" {
		payload["email"] = email
	}
	if err := audit.Emit(ctx, audit.AuthLoginFailed, audit.Record{
		Status:   audit.StatusFailure,
		Payload:  payload,
		ClientIP: GetIPAddress(r),
	}); err != nil {
		log.Printf("audit: emit %s: %v", audit.AuthLoginFailed.Type(), err)
	}
}

// exchangeDeviceCode does ONE GitHub device-code token exchange:
// (user,0,nil)=success; (nil,retryAfterSec,nil)=not authorised yet, re-poll;
// (nil,0,err)=terminal (denied / expired / misconfig). The client owns the loop
func exchangeDeviceCode(ctx context.Context, deviceCode string) (*LoggedUser, int, error) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		return nil, 0, fmt.Errorf("GITHUB_CLIENT_ID is not set")
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/json",
	}

	var tokenResp AccessTokenResponse
	if err := utils.Fetch(
		"POST",
		githubTokenURL,
		strings.NewReader(data.Encode()),
		headers,
		&tokenResp,
	); err != nil {
		return nil, 0, fmt.Errorf("failed to fetch access token: %w", err)
	}

	switch tokenResp.Error {
	case "":
		githubUser, err := fetchGitHubUser(tokenResp.AccessToken)
		if err != nil {
			return nil, 0, fmt.Errorf("token OK but failed to fetch user: %w", err)
		}
		if githubUser.Email == "" {
			return nil, 0, &signInRejectedError{reason: "no_verified_email"}
		}
		if !isEmailAllowed(githubUser.Email) {
			return nil, 0, &signInRejectedError{email: githubUser.Email, reason: "not_allowed"}
		}

		avatarURL := githubUser.AvatarURL
		githubID := db_types.NewJSONNullInt64(githubUser.ID)
		providerUserID := strconv.FormatInt(githubUser.ID, 10)

		// Look up by github identity (stable across email changes)
		identityRow, err := db.Queries.GetUserByProviderIdentity(ctx, generated.GetUserByProviderIdentityParams{
			Provider:       db_types.NewJSONNullString("github"),
			ProviderUserID: db_types.NewJSONNullString(providerUserID),
		})
		if err == nil {
			if identityRow.Email.String != githubUser.Email {
				if err := db.Queries.UpdateUserIdentityEmail(ctx, generated.UpdateUserIdentityEmailParams{
					Provider:       db_types.NewJSONNullString("github"),
					ProviderUserID: db_types.NewJSONNullString(providerUserID),
					Email:          db_types.NewJSONNullString(githubUser.Email),
				}); err != nil {
					return nil, 0, fmt.Errorf("failed to update identity email: %w", err)
				}
				if err := db.Queries.UpdateUserEmail(ctx, generated.UpdateUserEmailParams{
					ID:    identityRow.ID,
					Email: db_types.NewJSONNullString(githubUser.Email),
				}); err != nil {
					return nil, 0, fmt.Errorf("failed to update user email: %w", err)
				}
			}
			// Returning user: guarantee they still have a workspace. Idempotent
			// (no-op when they already have one), it recovers accounts whose
			// workspaces were all removed, which would otherwise dead-end login.
			if err := EnsureDefaultWorkspaceForUser(ctx, identityRow.ID.String()); err != nil {
				return nil, 0, fmt.Errorf("existing user default workspace setup failed: %w", err)
			}
			return &LoggedUser{ID: identityRow.ID, Name: identityRow.Name, AvatarURL: avatarURL}, 0, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, 0, fmt.Errorf("failed to look up identity: %w", err)
		}

		// Check for placeholder user (invited by email, no identity yet)
		placeholder, placeholderErr := db.Queries.GetUserByEmailNoIdentity(ctx, githubUser.Email)
		if placeholderErr == nil {
			if err := db.Queries.UpdateUserForLogin(ctx, generated.UpdateUserForLoginParams{
				ID:        placeholder.ID,
				Name:      db_types.NewJSONNullString(githubUser.Name),
				GithubID:  githubID,
				Email:     db_types.NewJSONNullString(githubUser.Email),
				AvatarUrl: db_types.NewJSONNullString(avatarURL),
			}); err != nil {
				return nil, 0, fmt.Errorf("failed to update placeholder user: %w", err)
			}
			if err := db.Queries.CreateUserIdentity(ctx, generated.CreateUserIdentityParams{
				UserID:         placeholder.ID,
				Provider:       db_types.NewJSONNullString("github"),
				ProviderUserID: db_types.NewJSONNullString(providerUserID),
				Email:          db_types.NewJSONNullString(githubUser.Email),
			}); err != nil {
				return nil, 0, fmt.Errorf("failed to create identity for placeholder user: %w", err)
			}
			if err := EnsureDefaultWorkspaceForUser(ctx, placeholder.ID.String()); err != nil {
				return nil, 0, fmt.Errorf("placeholder user resolved but default workspace setup failed: %w", err)
			}
			return &LoggedUser{ID: placeholder.ID, Name: db_types.NewJSONNullString(githubUser.Name), AvatarURL: avatarURL}, 0, nil
		}
		if !errors.Is(placeholderErr, sql.ErrNoRows) {
			return nil, 0, fmt.Errorf("failed to check for placeholder user: %w", placeholderErr)
		}

		// New user
		created, err := db.Queries.CreateUser(ctx, generated.CreateUserParams{
			Email:    db_types.NewJSONNullString(githubUser.Email),
			Name:     db_types.NewJSONNullString(githubUser.Name),
			GithubID: githubID,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create user: %w", err)
		}
		if err := db.Queries.CreateUserIdentity(ctx, generated.CreateUserIdentityParams{
			UserID:         created.ID,
			Provider:       db_types.NewJSONNullString("github"),
			ProviderUserID: db_types.NewJSONNullString(providerUserID),
			Email:          db_types.NewJSONNullString(githubUser.Email),
		}); err != nil {
			return nil, 0, fmt.Errorf("failed to create identity for new user: %w", err)
		}
		if err := EnsureDefaultWorkspaceForUser(ctx, created.ID.String()); err != nil {
			return nil, 0, fmt.Errorf("user created but default workspace setup failed: %w", err)
		}
		return &LoggedUser{ID: created.ID, Name: created.Name, AvatarURL: avatarURL}, 0, nil

	case "authorization_pending":
		return nil, 5, nil

	case "slow_down":
		n := tokenResp.Interval
		if n <= 0 {
			n = 10
		}
		return nil, n, nil

	case "access_denied":
		return nil, 0, errAuthDenied

	case "expired_token":
		return nil, 0, errCodeExpired

	default:
		desc := tokenResp.ErrorDesc
		if desc == "" {
			desc = tokenResp.Error
		}
		return nil, 0, fmt.Errorf("GitHub error: %s", desc)
	}
}
func fetchGitHubUser(accessToken string) (*GitHubUser, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
		"Accept":        "application/json",
	}

	var user GitHubUser
	err := utils.Fetch("GET", githubUserURL, nil, headers, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub user: %w", err)
	}

	if user.Email == "" {
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		err = utils.Fetch("GET", githubUserEmailsURL, nil, headers, &emails)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch GitHub user emails: %w", err)
		}

		for _, e := range emails {
			if e.Verified {
				user.Email = e.Email
				if e.Primary {
					break
				}
			}
		}
	}

	return &user, nil
}
