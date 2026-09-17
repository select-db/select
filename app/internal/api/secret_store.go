package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/zalando/go-keyring"

	"selectDb/internal/server"
)

const (
	keyringService  = "selectDb"
	keyringDeviceID = "device-id"
)

func tokenKey(kind, domain string) string {
	if domain == "" {
		return kind
	}
	return kind + ":" + domain
}

// SaveAccessToken saves the access token for the current server domain.
func SaveAccessToken(token string) error {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, tokenKey("access-token", domain), token)
}

// LoadAccessToken loads the access token for the current server domain.
func LoadAccessToken() (string, error) {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return "", err
	}
	return loadStringFromKeyring(tokenKey("access-token", domain))
}

// ClearAccessToken deletes the access token for the current server domain.
func ClearAccessToken() error {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return err
	}
	return keyring.Delete(keyringService, tokenKey("access-token", domain))
}

// SaveRefreshToken saves the refresh token for the current server domain.
func SaveRefreshToken(token string) error {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, tokenKey("refresh-token", domain), token)
}

// LoadRefreshToken loads the refresh token for the current server domain.
func LoadRefreshToken() (string, error) {
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return "", err
	}
	return loadStringFromKeyring(tokenKey("refresh-token", domain))
}

// ClearRefreshToken deletes the refresh token for the current server domain,
// in-process copy included, so a signed-out token is never presented on the
// session that follows.
func ClearRefreshToken() error {
	forgetRefreshToken()
	domain, err := server.ReadCurrentDomain()
	if err != nil {
		return err
	}
	return keyring.Delete(keyringService, tokenKey("refresh-token", domain))
}

// CredentialStatus is what the keyring can say about the stored session. The
// third case is the one that matters: a keyring that cannot be read has not
// said the user is signed out.
type CredentialStatus int

const (
	CredentialsUnreadable CredentialStatus = iota
	CredentialsPresent
	CredentialsMissing
)

// ReadCredentialStatus reports whether the current server's tokens are both
// there, positively gone, or unreadable. Sign-in and sign-out ask the same
// question and differ only in which answer they act on.
func ReadCredentialStatus() CredentialStatus {
	accessToken, accessErr := LoadAccessToken()
	refreshToken, refreshErr := LoadRefreshToken()
	switch {
	case accessErr == nil && refreshErr == nil && accessToken != "" && refreshToken != "":
		return CredentialsPresent
	case errors.Is(accessErr, keyring.ErrNotFound) || errors.Is(refreshErr, keyring.ErrNotFound):
		return CredentialsMissing
	default:
		return CredentialsUnreadable
	}
}

// LoadDeviceID loads the device ID string or generates/stores a new one if missing
func LoadDeviceID() (string, error) {
	deviceID, err := keyring.Get(keyringService, keyringDeviceID)
	if err == nil {
		if strings.ContainsRune(deviceID, '\x00') {
			return "", fmt.Errorf("device ID contains NUL byte")
		}
		return deviceID, nil
	}

	if errors.Is(err, keyring.ErrNotFound) {
		newDeviceID := uuid.NewString()
		if saveErr := keyring.Set(keyringService, keyringDeviceID, newDeviceID); saveErr != nil {
			return "", saveErr
		}
		return newDeviceID, nil
	}

	return "", err
}

// loadStringFromKeyring reads, checks for NUL, and returns value
func loadStringFromKeyring(key string) (string, error) {
	val, err := keyring.Get(keyringService, key)
	if err != nil {
		return "", err
	}

	if strings.ContainsRune(val, '\x00') {
		return "", fmt.Errorf("keyring value for %s contains NUL byte", key)
	}

	return val, nil
}
