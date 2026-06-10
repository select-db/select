package kms

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"
	okms "github.com/ovh/okms-sdk-go"
)

// KEKEnv is the dev-mode switch: a base64 (std) 32-byte KEK held in process.
// When set, providers and signers use local key material; otherwise they use
// OVH KMS (prod).
const KEKEnv = "SELECTDB_KEK"

// Shared OVH KMS connection env vars: endpoint, mTLS access cert/key, and the
// KMS instance ID. Per-key IDs live with their respective providers.
const (
	kmsEndpointEnv   = "OVH_KMS_ENDPOINT"
	kmsCertFileEnv   = "OVH_KMS_CERT_FILE"
	kmsKeyFileEnv    = "OVH_KMS_KEY_FILE"
	kmsInstanceIDEnv = "OVH_KMS_INSTANCE_ID"
)

// localMode reports whether the in-process dev KEK is configured. When true,
// providers and signers use local key material instead of OVH KMS.
func localMode() bool { return os.Getenv(KEKEnv) != "" }

// newKMSClient builds an mTLS-authenticated OVH KMS client from env. Shared by
// every prod provider and signer. Returns the KMS instance ID alongside the
// client (all keys are scoped to it).
func newKMSClient() (*okms.Client, uuid.UUID, error) {
	endpoint := os.Getenv(kmsEndpointEnv)
	certFile := os.Getenv(kmsCertFileEnv)
	keyFile := os.Getenv(kmsKeyFileEnv)
	if endpoint == "" || certFile == "" || keyFile == "" {
		return nil, uuid.Nil, fmt.Errorf("kms: no provider configured: set %s (dev) or %s/%s/%s (prod)",
			KEKEnv, kmsEndpointEnv, kmsCertFileEnv, kmsKeyFileEnv)
	}
	kmsID, err := uuid.Parse(os.Getenv(kmsInstanceIDEnv))
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("kms: %s invalid: %w", kmsInstanceIDEnv, err)
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("kms: load kms cert: %w", err)
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		},
	}
	client, err := okms.NewRestAPIClientWithHttp(endpoint, httpClient)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("kms: kms client: %w", err)
	}
	return client, kmsID, nil
}
