package kms

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	okms "github.com/ovh/okms-sdk-go"
	"golang.org/x/crypto/chacha20poly1305"
)

// kmsKEKIDEnv names the OVH service key (symmetric) used as the envelope KEK.
const kmsKEKIDEnv = "OVH_KMS_KEK_ID"

// KEKProvider mints and unwraps DEKs under a KEK it controls; the KEK never
// leaves the provider. AAD binding lives at the payload layer (see Wrapper).
type KEKProvider interface {
	// KeyID identifies the KEK. Embedded in every blob for unwrap routing and
	// rotation. Must be <= 255 bytes.
	KeyID() string

	// NewDEK returns a fresh data key: plain for immediate use, wrapped for
	// storage in the blob. Caller must zero plain after use.
	NewDEK(ctx context.Context) (plain, wrapped []byte, err error)

	UnwrapDEK(ctx context.Context, wrapped []byte) (plain []byte, err error)
}

// NewKEKProvider selects the envelope KEK provider: the in-process KEK if
// SELECTDB_KEK is set (dev), otherwise OVH KMS (prod).
func NewKEKProvider() (KEKProvider, error) {
	if localMode() {
		return newLocalProvider(KEKEnv)
	}
	return newOKMSProvider()
}

// localProvider holds the KEK in process. Dev/local only; prod uses OVH KMS.
type localProvider struct {
	kek   []byte
	keyID string
}

func newLocalProvider(envVar string) (*localProvider, error) {
	raw := os.Getenv(envVar)
	if raw == "" {
		return nil, fmt.Errorf("kms: %s not set", envVar)
	}
	kek, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("kms: %s not valid base64: %w", envVar, err)
	}
	if len(kek) != dekSize {
		return nil, fmt.Errorf("kms: %s must be %d bytes, got %d", envVar, dekSize, len(kek))
	}
	// keyID derives from the KEK hash so rotating the key changes the ID.
	sum := sha256.Sum256(kek)
	return &localProvider{kek: kek, keyID: "env:" + hex.EncodeToString(sum[:4])}, nil
}

func (p *localProvider) KeyID() string { return p.keyID }

func (p *localProvider) NewDEK(_ context.Context) (plain, wrapped []byte, err error) {
	dek := make([]byte, dekSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, nil, err
	}
	aead, err := chacha20poly1305.NewX(p.kek)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	sealed := aead.Seal(nil, nonce, dek, nil)
	wrapped = make([]byte, 0, nonceSize+len(sealed))
	wrapped = append(wrapped, nonce...)
	wrapped = append(wrapped, sealed...)
	return dek, wrapped, nil
}

func (p *localProvider) UnwrapDEK(_ context.Context, wrapped []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(p.kek)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < nonceSize {
		return nil, errors.New("kms: wrapped dek too short")
	}
	return aead.Open(nil, wrapped[:nonceSize], wrapped[nonceSize:], nil)
}

// dataKeyBits is the DEK size requested from OVH KMS. 256 -> 32 bytes, matching
// chacha20poly1305.KeySize used at the payload layer.
const dataKeyBits = 256

// dataKeyName labels generated data keys in the KMS audit log.
const dataKeyName = "datasource-dek"

// okmsProvider delegates DEK generation/unwrapping to OVH KMS. The KEK never
// leaves the KMS. Auth is mTLS via an access certificate.
type okmsProvider struct {
	dk    *okms.DataKeyProvider
	keyID string
}

// newOKMSProvider builds the envelope provider. kekID is the OVH service key
// (symmetric) used as the KEK protecting the data keys.
func newOKMSProvider() (*okmsProvider, error) {
	client, okmsID, err := newOKMSClient()
	if err != nil {
		return nil, err
	}
	kekID, err := uuid.Parse(os.Getenv(kmsKEKIDEnv))
	if err != nil {
		return nil, fmt.Errorf("kms: %s invalid: %w", kmsKEKIDEnv, err)
	}
	return &okmsProvider{
		dk:    client.DataKeys(okmsID, kekID),
		keyID: "ovh:" + kekID.String(),
	}, nil
}

func (p *okmsProvider) KeyID() string { return p.keyID }

func (p *okmsProvider) NewDEK(ctx context.Context) (plain, wrapped []byte, err error) {
	return p.dk.GenerateDataKey(ctx, dataKeyName, dataKeyBits)
}

func (p *okmsProvider) UnwrapDEK(ctx context.Context, wrapped []byte) ([]byte, error) {
	return p.dk.DecryptDataKey(ctx, wrapped)
}
