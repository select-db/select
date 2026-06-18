package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostKeyLine resolves a bastion's host key from ~/.ssh/known_hosts and
// returns it as an authorized-key line ("host TYPE BASE64"), or "" when the
// host is not recorded (or known_hosts is unreadable). Desktop only: it lets
// SELECT reuse the trust decision the user already made with their own ssh
// client, so no manual paste is needed. The proxy never takes this path —
// EnforceOutboundGuard keeps requiring an explicitly configured key.
func knownHostKeyLine(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port == 0 {
		port = 22
	}
	key, err := lookupKnownHost(host, port)
	if err != nil || key == nil {
		return ""
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	return host + " " + line // ssh-keyscan style; parses via ParseAuthorizedKey
}

// lookupKnownHost returns the pinned key for host:port from ~/.ssh/known_hosts,
// or (nil, nil) when the host is not recorded. Any other situation (missing
// file, parse error) returns a non-nil error.
func lookupKnownHost(host string, port int) (ssh.PublicKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}

	// knownhosts has no direct lookup; probe with a throwaway key and read the
	// recorded keys back out of the returned KeyError.Want.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	// The callback requires a SplitHostPort-parseable "host:port" address; it
	// normalizes internally, so pass the raw join rather than Normalize's output.
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = cb(addr, &net.TCPAddr{IP: net.IPv4zero, Port: port}, signer.PublicKey())
	if err == nil {
		return nil, nil // throwaway key matched a record: treat as not found
	}
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) {
		if len(keyErr.Want) > 0 {
			return keyErr.Want[0].Key, nil
		}
		return nil, nil // host absent from known_hosts
	}
	return nil, err
}
