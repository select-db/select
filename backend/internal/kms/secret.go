package kms

import (
	"context"
	"fmt"
	"os"
)

// secretField is the data field every managed secret is stored under in the
// OVH KMS secret store.
const secretField = "value"

// Secret resolves a named secret. Dev sets the env var <name> directly; prod
// leaves it unset and stores the value in the OVH KMS secret store at path
// <name>, field "value". Convention: the env var name IS the secret path, so a
// single identifier drives both lookups.
//
// The value is plaintext in process by necessity (callers need it, e.g. a DSN
// for sql.Open), so KMS protects it at rest, not in memory.
func Secret(name string) (string, error) {
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	return fetchSecret(name, secretField)
}

// fetchSecret reads a single field from a secret in the OVH KMS secret store.
func fetchSecret(path, key string) (string, error) {
	client, okmsID, err := newOKMSClient()
	if err != nil {
		return "", err
	}
	includeData := true
	resp, err := client.GetSecretV2(context.Background(), okmsID, path, nil, &includeData)
	if err != nil {
		return "", fmt.Errorf("kms: fetch secret %q: %w", path, err)
	}
	if resp.Version == nil || resp.Version.Data == nil {
		return "", fmt.Errorf("kms: secret %q has no data", path)
	}
	v, ok := (*resp.Version.Data)[key]
	if !ok {
		return "", fmt.Errorf("kms: secret %q missing key %q", path, key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("kms: secret %q key %q is not a string", path, key)
	}
	return s, nil
}
