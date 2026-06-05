package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSSHHostKeyCallback(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pub)
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	// Valid pinned key -> strict callback + algorithm, no error.
	if r, err := sshHostKeyCallback(authorized); err != nil || r.callback == nil {
		t.Fatalf("valid host key: result=%v err=%v, want non-nil cb, nil err", r, err)
	} else if r.algorithm != sshPub.Type() {
		t.Fatalf("algorithm = %q, want %q", r.algorithm, sshPub.Type())
	}

	// Garbage -> error.
	if _, err := sshHostKeyCallback("not-a-key"); err == nil {
		t.Error("garbage host key should error")
	}

	// Empty + proxy guard ON -> fail closed (no silent MITM).
	EnforceOutboundGuard = true
	if _, err := sshHostKeyCallback(""); err == nil {
		t.Error("proxy with no host key must fail closed")
	}
	if _, err := sshHostKeyCallback("   "); err == nil {
		t.Error("proxy with blank host key must fail closed")
	}
	EnforceOutboundGuard = false

	// Empty + desktop (guard OFF) -> TOFU-style accept, no error, no algorithm preference.
	if r, err := sshHostKeyCallback(""); err != nil || r.callback == nil {
		t.Fatalf("desktop with no host key: result=%v err=%v, want accept", r, err)
	} else if r.algorithm != "" {
		t.Fatalf("desktop should have no algorithm preference, got %q", r.algorithm)
	}
}
