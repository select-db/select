package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// An encrypted pasted key has no passphrase channel (unlike key_file), so
// buildSSHAuth must fail with a clear, actionable message rather than the raw
// "this private key is passphrase protected".
func TestBuildSSHAuthEncryptedInlineKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(block))

	_, err = buildSSHAuth(ResolvedSSHConfig{AuthMethod: "private_key", PrivateKey: pemKey})
	if err == nil {
		t.Fatal("encrypted inline key should error")
	}
	if !strings.Contains(err.Error(), "passphrase-protected") {
		t.Fatalf("error = %q, want it to mention passphrase-protected", err)
	}
}
