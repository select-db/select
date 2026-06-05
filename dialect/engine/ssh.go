package engine

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ResolvedSSHConfig holds SSH connection settings after variable substitution.
// The app resolves variables and builds this struct before passing it to
// StartSSHTunnel; the engine never reads .env or graph types.
type ResolvedSSHConfig struct {
	Host       string
	Port       int
	User       string
	AuthMethod string // "password" | "private_key"
	Password   string
	PrivateKey string
	HostKey    string // expected host public key ("TYPE BASE64"); pins the bastion
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
		return hostKeyResult{callback: ssh.InsecureIgnoreHostKey()}, nil // #nosec G106 -- desktop-only trust-on-first-use; the proxy (EnforceOutboundGuard) requires a pinned host key
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

	authMethod, err := buildSSHAuth(config)
	if err != nil {
		return nil, err
	}

	hkResult, err := sshHostKeyCallback(config.HostKey)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            []ssh.AuthMethod{authMethod},
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

func buildSSHAuth(config ResolvedSSHConfig) (ssh.AuthMethod, error) {
	switch strings.ToLower(config.AuthMethod) {
	case "password":
		if config.Password == "" {
			return nil, newConfigError("ssh password auth selected but password is empty")
		}
		return ssh.Password(config.Password), nil
	case "private_key":
		keyText := strings.TrimSpace(config.PrivateKey)
		if keyText == "" {
			return nil, newConfigError("ssh private key auth selected but private key is empty")
		}
		signer, err := ssh.ParsePrivateKey([]byte(keyText))
		if err != nil {
			return nil, newConfigErrorf("ssh private key parse error: %v", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, newConfigErrorf("unsupported ssh auth method: %s", config.AuthMethod)
	}
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
