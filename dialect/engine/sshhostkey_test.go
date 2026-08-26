package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHHostKeyCallback(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pub)
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	// Valid pinned key -> strict callback + single matching algorithm, no error.
	if r, err := sshHostKeyCallback(authorized); err != nil || r.callback == nil {
		t.Fatalf("valid host key: result=%v err=%v, want non-nil cb, nil err", r, err)
	} else if len(r.algorithms) != 1 || r.algorithms[0] != sshPub.Type() {
		t.Fatalf("algorithms = %v, want [%q]", r.algorithms, sshPub.Type())
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
	} else if len(r.algorithms) != 0 {
		t.Fatalf("desktop should have no algorithm preference, got %v", r.algorithms)
	}
}

// TestHostKeyAlgorithmsFor: a pinned RSA host key must also offer the SHA-2
// signature variants, else negotiation fails against a modern OpenSSH server
// that no longer advertises legacy SHA-1 "ssh-rsa".
func TestHostKeyAlgorithmsFor(t *testing.T) {
	got := hostKeyAlgorithmsFor(ssh.KeyAlgoRSA)
	want := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	if len(got) != len(want) {
		t.Fatalf("rsa algorithms = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rsa algorithms = %v, want %v", got, want)
		}
	}

	// Non-RSA keys pin exactly their own type.
	if got := hostKeyAlgorithmsFor(ssh.KeyAlgoED25519); len(got) != 1 || got[0] != ssh.KeyAlgoED25519 {
		t.Fatalf("ed25519 algorithms = %v, want [%q]", got, ssh.KeyAlgoED25519)
	}
}

// newTestHostKey returns a fresh ed25519 ssh public key.
func newTestHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

// TestKnownHostKeyLine: a key recorded in ~/.ssh/known_hosts is returned as a
// pinnable authorized-key line; an unrecorded host yields "".
func TestKnownHostKeyLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	pub := newTestHostKey(t)
	const host = "bastion.example.com"
	addr := knownhosts.Normalize(net.JoinHostPort(host, "22"))
	line := knownhosts.Line([]string{addr}, pub)
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Recorded host -> pinnable line that parses back to the same key.
	got := knownHostKeyLine(host, 22)
	if got == "" {
		t.Fatal("recorded host returned empty line")
	}
	parsed, _, _, _, perr := ssh.ParseAuthorizedKey([]byte(got))
	if perr != nil {
		t.Fatalf("pinned line does not parse: %v", perr)
	}
	if ssh.FingerprintSHA256(parsed) != ssh.FingerprintSHA256(pub) {
		t.Error("pinned key does not match the recorded key")
	}

	// Unrecorded host -> empty (engine falls back to insecure on desktop).
	if other := knownHostKeyLine("not-recorded.example.com", 22); other != "" {
		t.Errorf("unrecorded host returned %q, want empty", other)
	}
}
