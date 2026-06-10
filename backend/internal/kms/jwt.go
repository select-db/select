package kms

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// kmsJWTKeyIDEnv names the OVH asymmetric key used to sign JWTs.
const kmsJWTKeyIDEnv = "OVH_KMS_JWT_KEY_ID"

// NewJWTSigner returns the JWT signing key as a crypto.Signer. Dev loads an RSA
// key from pemPath and holds it in process; prod uses the OVH KMS asymmetric
// key named by OVH_KMS_JWT_KEY_ID, whose private half never leaves the KMS.
// Either way the verifier reads the public half via signer.Public().
func NewJWTSigner(pemPath string) (crypto.Signer, error) {
	if localMode() {
		return loadLocalSigner(pemPath)
	}
	client, okmsID, err := newOKMSClient()
	if err != nil {
		return nil, err
	}
	keyID, err := uuid.Parse(os.Getenv(kmsJWTKeyIDEnv))
	if err != nil {
		return nil, fmt.Errorf("kms: %s invalid: %w", kmsJWTKeyIDEnv, err)
	}
	return client.NewSigner(context.Background(), okmsID, keyID)
}

// loadLocalSigner parses an RSA private key (PKCS#1 or PKCS#8) from a PEM file.
func loadLocalSigner(pemPath string) (crypto.Signer, error) {
	if pemPath == "" {
		return nil, fmt.Errorf("kms: signing key path not set")
	}
	raw, err := os.ReadFile(pemPath) // #nosec G304 -- operator-set path, not user input
	if err != nil {
		return nil, fmt.Errorf("kms: read signing key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("kms: signing key is not PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("kms: parse signing key: %w", err)
	}
	signer, ok := keyAny.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("kms: signing key is not a crypto.Signer")
	}
	return signer, nil
}
