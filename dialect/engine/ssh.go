package engine

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ResolvedSSHConfig holds SSH connection settings after variable substitution.
// The app resolves variables and builds this struct before passing it to
// StartSSHTunnel; the engine never reads .env or graph types.
//
// AuthMethod "agent" and "key_file" are desktop-only: they read the user's
// SSH agent / a key file on the local machine and are rejected when the
// outbound guard is on (the proxy server cannot reach the user's machine).
type ResolvedSSHConfig struct {
	Host       string
	Port       int
	User       string
	AuthMethod string // "password" | "private_key" | "agent" | "key_file"
	Password   string
	PrivateKey string // inline key material (proxified / private_key)
	KeyPath    string // path to a private key file on disk (desktop key_file)
	Passphrase string // in-memory only; decrypts an encrypted key_file. Never persisted.
	HostKey    string // expected host public key ("[host ]TYPE BASE64"); pins the bastion
}

type hostKeyResult struct {
	callback  ssh.HostKeyCallback
	algorithm string // empty = no preference
}

// sshHostKeyCallback pins the bastion's host key. When a key is configured it
// is verified strictly and the algorithm is locked to match. When absent the
// proxy (EnforceOutboundGuard) fails closed; the desktop app keeps
// trust-on-first-use behaviour.
func sshHostKeyCallback(hostKey string) (hostKeyResult, error) {
	hk := strings.TrimSpace(hostKey)
	if hk == "" {
		if EnforceOutboundGuard {
			return hostKeyResult{}, newConfigError("ssh host key required: configure the SSH host key for this datasource")
		}
		return hostKeyResult{callback: ssh.InsecureIgnoreHostKey()}, nil // #nosec G106 nosemgrep: go.lang.security.audit.crypto.insecure_ssh.avoid-ssh-insecure-ignore-host-key -- desktop-only trust-on-first-use; the proxy (EnforceOutboundGuard) fails closed above and requires a pinned host key
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hk))
	if err != nil {
		return hostKeyResult{}, newConfigErrorf("invalid ssh host key (expected \"type base64\", e.g. ssh-keyscan output): %v", err)
	}
	return hostKeyResult{
		callback:  ssh.FixedHostKey(pub),
		algorithm: pub.Type(),
	}, nil
}

// sshTunnel is a single SSH local port-forward. It implements Tunnel.
type sshTunnel struct {
	localAddr  string
	remoteHost string
	remotePort int

	client   *ssh.Client
	listener net.Listener
	once     sync.Once
}

// Close implements Tunnel.
func (tunnel *sshTunnel) Close() {
	tunnel.once.Do(func() {
		if tunnel.listener != nil {
			_ = tunnel.listener.Close()
		}
		if tunnel.client != nil {
			_ = tunnel.client.Close()
		}
	})
}

// IsAlive implements Tunnel. Sends a lightweight keepalive; returns false on
// a broken connection.
func (tunnel *sshTunnel) IsAlive() bool {
	if tunnel.client == nil {
		return false
	}
	_, _, err := tunnel.client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

// LocalAddr implements Tunnel.
func (tunnel *sshTunnel) LocalAddr() string {
	return tunnel.localAddr
}

// LocalPort implements Tunnel.
func (tunnel *sshTunnel) LocalPort() (int, error) {
	_, portStr, err := net.SplitHostPort(tunnel.localAddr)
	if err != nil {
		return 0, fmt.Errorf("parse local addr %q: %w", tunnel.localAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("parse port %q: %w", portStr, err)
	}
	return port, nil
}

// StartSSHTunnel establishes an SSH client and a local TCP listener that
// forwards connections to remoteHost:remotePort through the SSH connection.
func StartSSHTunnel(config ResolvedSSHConfig, remoteHost string, remotePort int) (Tunnel, error) {
	if config.Host == "" {
		return nil, newConfigError("ssh host is required")
	}
	if config.User == "" {
		return nil, newConfigError("ssh user is required")
	}
	if err := validateOutboundHost(config.Host); err != nil {
		return nil, err
	}

	auth, err := buildSSHAuth(config)
	if err != nil {
		return nil, err
	}
	// The agent socket (if any) is only needed while ssh.Dial performs the
	// handshake below; release it once this function returns.
	defer auth.cleanup()

	// Desktop convenience: when no key is configured, reuse the user's existing
	// trust by pinning the bastion from ~/.ssh/known_hosts. Falls through to the
	// (desktop-only) insecure path when the host is unknown. The proxy never
	// reaches here with an empty key, EnforceOutboundGuard fails closed below.
	hostKey := config.HostKey
	if strings.TrimSpace(hostKey) == "" && !EnforceOutboundGuard {
		hostKey = knownHostKeyLine(config.Host, config.Port)
	}

	hkResult, err := sshHostKeyCallback(hostKey)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            []ssh.AuthMethod{auth.method},
		HostKeyCallback: hkResult.callback,
		Timeout:         5 * time.Second,
	}
	if hkResult.algorithm != "" {
		sshConfig.HostKeyAlgorithms = []string{hkResult.algorithm}
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	// Preflight: confirm the bastion can actually reach the database. Without
	// this, an unreachable DB (wrong DSN host/port, or TCP forwarding disabled on
	// the bastion) only surfaces later as an opaque "connection reset by peer" on
	// the forwarded local socket.
	target := fmt.Sprintf("%s:%d", remoteHost, remotePort)
	probe, derr := client.Dial("tcp", target)
	if derr != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ssh tunnel to %s is up, but the database at %s is not reachable through it: %w (check the DSN host/port are reachable from the bastion, and that the bastion allows TCP forwarding)", addr, target, derr)
	}
	_ = probe.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ssh local listen: %w", err)
	}

	tunnel := &sshTunnel{
		localAddr:  ln.Addr().String(),
		remoteHost: remoteHost,
		remotePort: remotePort,
		client:     client,
		listener:   ln,
	}
	go tunnel.serve()
	return tunnel, nil
}

// sshAuth pairs an auth method with a cleanup that releases any resource the
// method holds (e.g. the SSH-agent socket). cleanup is always non-nil and must
// be called once the handshake is done. It is a no-op for stateless methods.
type sshAuth struct {
	method  ssh.AuthMethod
	cleanup func()
}

func noCleanup() {}

// buildSSHAuth selects the auth method for the configured AuthMethod.
func buildSSHAuth(config ResolvedSSHConfig) (sshAuth, error) {
	switch strings.ToLower(config.AuthMethod) {
	case "password":
		if config.Password == "" {
			return sshAuth{}, newConfigError("ssh password auth selected but password is empty")
		}
		return sshAuth{method: ssh.Password(config.Password), cleanup: noCleanup}, nil
	case "private_key":
		keyText := strings.TrimSpace(config.PrivateKey)
		if keyText == "" {
			return sshAuth{}, newConfigError("ssh private key auth selected but private key is empty")
		}
		signer, err := ssh.ParsePrivateKey([]byte(keyText))
		if err != nil {
			return sshAuth{}, newConfigErrorf("ssh private key parse error: %v", err)
		}
		return sshAuth{method: ssh.PublicKeys(signer), cleanup: noCleanup}, nil
	case "agent":
		return agentAuth()
	case "key_file":
		method, err := keyFileAuth(config.KeyPath, config.Passphrase)
		if err != nil {
			return sshAuth{}, err
		}
		return sshAuth{method: method, cleanup: noCleanup}, nil
	default:
		return sshAuth{}, newConfigErrorf("unsupported ssh auth method: %s", config.AuthMethod)
	}
}

// agentAuth authenticates via the running SSH agent. Desktop only: nothing is
// stored and encrypted/hardware keys are handled by the agent. Rejected when the
// outbound guard is on. The agent connection is only queried during the
// handshake, so cleanup closes it once StartSSHTunnel returns. The platform
// transport (unix socket vs Windows named pipe) is resolved by dialAgent.
func agentAuth() (sshAuth, error) {
	if EnforceOutboundGuard {
		return sshAuth{}, newConfigError("ssh agent auth is not available for proxified connections")
	}
	conn, err := dialAgent()
	if err != nil {
		return sshAuth{}, newConfigErrorf("ssh agent: %v", err)
	}
	return sshAuth{
		method:  ssh.PublicKeysCallback(agent.NewClient(conn).Signers),
		cleanup: func() { _ = conn.Close() },
	}, nil
}

// keyFileAuth reads a private key from a path on the local machine. Desktop
// only: the path (not the contents) is persisted, and the key is read at
// connect time. Rejected when the outbound guard is on.
func keyFileAuth(path, passphrase string) (ssh.AuthMethod, error) {
	if EnforceOutboundGuard {
		return nil, newConfigError("ssh key-file auth is not available for proxified connections; paste the key instead")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, newConfigError("ssh key-file auth selected but no key file chosen")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- desktop: user explicitly picks this path
	if err != nil {
		return nil, newConfigErrorf("read ssh key file %q: %v", path, err)
	}
	// Try unencrypted first. Only an encrypted key needs the passphrase; this
	// way a stale/leftover passphrase on an unencrypted key is harmless (ignored)
	// rather than being mis-reported as "passphrase incorrect".
	signer, err := ssh.ParsePrivateKey(data)
	var missing *ssh.PassphraseMissingError
	switch {
	case errors.As(err, &missing):
		if passphrase == "" {
			return nil, newConfigError("ssh key file is encrypted: enter its passphrase")
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			// Wrong passphrase. Stable "passphrase" message so the app re-prompts.
			return nil, newConfigError("ssh key passphrase incorrect: re-enter the passphrase")
		}
	case err != nil:
		return nil, newConfigErrorf("parse ssh key file: %v", err)
	}
	return ssh.PublicKeys(signer), nil
}

func (tunnel *sshTunnel) serve() {
	defer func() { recover() }() //nolint:errcheck
	for {
		connection, err := tunnel.listener.Accept()
		if err != nil {
			return
		}
		go func(localConn net.Conn) {
			defer func() { recover() }() //nolint:errcheck
			defer func() { _ = localConn.Close() }()

			remote, err := tunnel.client.Dial("tcp", fmt.Sprintf("%s:%d", tunnel.remoteHost, tunnel.remotePort))
			if err != nil {
				return
			}
			defer func() { _ = remote.Close() }()

			go func() { _, _ = io.Copy(remote, localConn) }()
			_, _ = io.Copy(localConn, remote)
		}(connection)
	}
}
