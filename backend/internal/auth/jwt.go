package auth

import (
	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/kms"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	issuer          = "selectdb"
	audience        = "selectdb-client"
	accessTokenTTL  = 5 * time.Minute
	refreshTokenTTL = 10 * 24 * time.Hour
)

// JWT signing key. In dev the RSA key is loaded from PRIVATE_KEY_PATH and held
// in process; in prod it is an OVH KMS asymmetric key (private half never leaves
// the KMS), reached through a crypto.Signer. The public half for verification is
// derived from the signer, so PUBLIC_KEY_PATH is no longer needed.
var (
	jwtSigner crypto.Signer
	publicKey *rsa.PublicKey
	keyOnce   sync.Once
	keyErr    error
)

func loadSigner() {
	s, err := kms.NewJWTSigner(os.Getenv("PRIVATE_KEY_PATH"))
	if err != nil {
		keyErr = fmt.Errorf("failed to load JWT signer: %w", err)
		return
	}
	pub, ok := s.Public().(*rsa.PublicKey)
	if !ok {
		keyErr = fmt.Errorf("JWT signer public key is not RSA")
		return
	}
	jwtSigner = s
	publicKey = pub
}

func getSigner() (crypto.Signer, error) {
	keyOnce.Do(loadSigner)
	return jwtSigner, keyErr
}

func getPublicKey() (*rsa.PublicKey, error) {
	keyOnce.Do(loadSigner)
	return publicKey, keyErr
}

// rsaSignerMethod adapts a crypto.Signer (local RSA key or remote KMS key) to
// golang-jwt. It emits standard RS256 signatures, so tokens verify with the
// stock SigningMethodRS256 against the public key.
type rsaSignerMethod struct{}

func (rsaSignerMethod) Alg() string { return "RS256" }

func (rsaSignerMethod) Sign(signingString string, key any) ([]byte, error) {
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, jwt.ErrInvalidKeyType
	}
	h := sha256.Sum256([]byte(signingString))
	return signer.Sign(rand.Reader, h[:], crypto.SHA256)
}

func (rsaSignerMethod) Verify(signingString string, sig []byte, key any) error {
	return jwt.SigningMethodRS256.Verify(signingString, sig, key)
}

var jwtSigningMethod = rsaSignerMethod{}

// Name is cached at issuance so audit/UI need no per-request role lookup.
type RoleRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Roles are workspace-scoped, so the token groups them per workspace.
type WorkspaceClaim struct {
	ID      string    `json:"id"`
	IsOwner bool      `json:"is_owner,omitempty"`
	Roles   []RoleRef `json:"roles,omitempty"`
}

// Roles/ownership are cached at issuance (a role change revokes the refresh
// token, forcing a fresh one). Membership itself is re-derived from the DB per
// request, not trusted from the token.
type CustomClaims struct {
	UserID     string           `json:"sub"`
	Name       string           `json:"name,omitempty"`
	Workspaces []WorkspaceClaim `json:"workspaces,omitempty"`
	jwt.RegisteredClaims
}

// CreateJWT issues a signed access token embedding per-workspace roles/ownership.
func CreateJWT(ctx context.Context, userID db_types.JSONNullUUID) (string, error) {
	signer, err := getSigner()
	if err != nil {
		return "", fmt.Errorf("unable to sign JWT: %w", err)
	}

	displayName := ""
	var workspaces []WorkspaceClaim
	if db.Queries != nil {
		// Membership, ownership, roles — grouped per workspace below.
		var workspaceIDs []string
		if ids, err := db.Queries.GetWorkspaceIDsByUserID(ctx, userID); err == nil {
			for _, u := range ids {
				if u.Valid {
					workspaceIDs = append(workspaceIDs, u.UUID.String())
				}
			}
		}

		owned := map[string]bool{}
		if ids, err := db.Queries.GetOwnedWorkspaceIDsByUserID(ctx, userID); err == nil {
			for _, u := range ids {
				if u.Valid {
					owned[u.UUID.String()] = true
				}
			}
		}

		rolesByWS := map[string][]RoleRef{}
		if rows, err := db.Queries.GetUserRolesWithNames(ctx, userID); err == nil {
			for _, r := range rows {
				if r.ID.Valid && r.WorkspaceID.Valid {
					ws := r.WorkspaceID.UUID.String()
					rolesByWS[ws] = append(rolesByWS[ws], RoleRef{ID: r.ID.UUID.String(), Name: r.Name.ValueOrEmpty()})
				}
			}
		}

		workspaces = make([]WorkspaceClaim, 0, len(workspaceIDs))
		for _, ws := range workspaceIDs {
			workspaces = append(workspaces, WorkspaceClaim{
				ID:      ws,
				IsOwner: owned[ws],
				Roles:   rolesByWS[ws],
			})
		}

		if u, err := db.Queries.GetUserNameByID(ctx, userID); err == nil {
			displayName = u.Name.ValueOrEmpty()
			if displayName == "" {
				displayName = u.Email.ValueOrEmpty()
			}
		}
	}

	claims := CustomClaims{
		UserID:     userID.UUID.String(),
		Name:       displayName,
		Workspaces: workspaces,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwtSigningMethod, claims)
	return token.SignedString(signer)
}

// CreateRefreshToken creates a refresh token, stores the hashed version in DB.
// Runs in a transaction: insert first, then delete old tokens, so a failed insert
// does not leave the user with no refresh tokens. ctx is used for DB calls so creation
// can be cancelled if the client disconnects.
func CreateRefreshToken(ctx context.Context, userID db_types.JSONNullUUID, deviceID string, issuedIP string) (*string, error) {
	plainToken := GenerateRandomString(64)
	expiry := time.Now().Add(refreshTokenTTL)
	hashedToken := HashRefreshToken(plainToken, deviceID)

	ip, err := toPgInet(issuedIP)
	if err != nil {
		return nil, fmt.Errorf("invalid IP: %w", err)
	}

	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	q := db.Queries.WithTx(tx)
	err = q.CreateRefreshToken(ctx, generated.CreateRefreshTokenParams{
		UserID:      userID,
		HashedToken: db_types.NewJSONNullString(hashedToken),
		ExpiresAt:   db_types.NewJSONNullTimeFromTime(expiry),
		IssuedIp:    db_types.NewJSONNullInet(ip),
	})
	if err != nil {
		return nil, err
	}
	err = q.DeleteUserRefreshTokensExcept(ctx, generated.DeleteUserRefreshTokensExceptParams{
		UserID:      userID,
		HashedToken: db_types.NewJSONNullString(hashedToken),
	})
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &plainToken, nil
}

// ValidateJWT verifies a JWT and returns its claims if valid
func ValidateJWT(tokenStr string) (*jwt.Token, *CustomClaims, error) {
	pubKey, err := getPublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to validate JWT: %w", err)
	}

	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure signing method is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	}, jwt.WithAudience(audience), jwt.WithIssuer(issuer))

	if err != nil {
		// When the token is expired, the library still parses and fills claims;
		// return them so the middleware can refresh using claims.UserID.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return token, claims, err
		}
		return nil, nil, err
	}
	return token, claims, nil
}
