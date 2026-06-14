package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// TestBuildSSHAuthAgent covers the agent auth path: rejected under the outbound
// guard, rejected with no SSH_AUTH_SOCK, and accepted against a live agent.
func TestBuildSSHAuthAgent(t *testing.T) {
	EnforceOutboundGuard = true
	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "agent"}); err == nil {
		t.Error("agent auth under outbound guard must be rejected")
	}
	EnforceOutboundGuard = false

	t.Setenv("SSH_AUTH_SOCK", "")
	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "agent"}); err == nil {
		t.Error("agent auth with no SSH_AUTH_SOCK must error")
	}

	// Live in-process agent over a unix socket.
	dir := t.TempDir()
	sock := filepath.Join(dir, "a.sock")
	if len(sock) > 100 {
		t.Skipf("socket path too long for unix domain socket: %s", sock)
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	keyring := agent.NewKeyring()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { _ = agent.ServeAgent(keyring, c) }(c)
		}
	}()

	t.Setenv("SSH_AUTH_SOCK", sock)
	auth, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "agent"})
	if err != nil || auth.method == nil {
		t.Fatalf("agent auth against live agent: method=%v err=%v", auth.method, err)
	}
}

// TestBuildSSHAuthKeyFile covers the key_file auth path: guard rejection, empty
// path, plaintext key, and encrypted key with/without the passphrase.
func TestBuildSSHAuthKeyFile(t *testing.T) {
	EnforceOutboundGuard = true
	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file", KeyPath: "/nope"}); err == nil {
		t.Error("key_file auth under outbound guard must be rejected")
	}
	EnforceOutboundGuard = false

	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file"}); err == nil {
		t.Error("key_file auth with empty path must error")
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	// Plaintext key.
	blk, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	plainPath := filepath.Join(dir, "id_plain")
	if err := os.WriteFile(plainPath, pem.EncodeToMemory(blk), 0o600); err != nil {
		t.Fatal(err)
	}
	if auth, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file", KeyPath: plainPath}); err != nil || auth.method == nil {
		t.Fatalf("plaintext key file: method=%v err=%v", auth.method, err)
	}

	// Encrypted key.
	encBlk, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("s3cret"))
	if err != nil {
		t.Fatal(err)
	}
	encPath := filepath.Join(dir, "id_enc")
	if err := os.WriteFile(encPath, pem.EncodeToMemory(encBlk), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file", KeyPath: encPath}); err == nil {
		t.Error("encrypted key without passphrase must error")
	}
	// Wrong passphrase: error must mention "passphrase" so the app re-prompts.
	if _, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file", KeyPath: encPath, Passphrase: "wrong"}); err == nil {
		t.Error("encrypted key with wrong passphrase must error")
	} else if !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("wrong-passphrase error = %q, want it to mention \"passphrase\"", err)
	}
	if auth, err := buildSSHAuth(ResolvedSSHConfig{AuthMethod: "key_file", KeyPath: encPath, Passphrase: "s3cret"}); err != nil || auth.method == nil {
		t.Fatalf("encrypted key with passphrase: method=%v err=%v", auth.method, err)
	}
}
