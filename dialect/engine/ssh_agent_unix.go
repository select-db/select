//go:build !windows

package engine

import (
	"errors"
	"io"
	"net"
	"os"
)

// dialAgent connects to the SSH agent via the SSH_AUTH_SOCK unix socket
// (macOS, Linux, BSD).
func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set (no agent running)")
	}
	// sock is the user's own SSH_AUTH_SOCK (a local unix socket to their agent),
	// not an attacker-controlled network address.
	return net.Dial("unix", sock) // #nosec G704 -- local agent socket from SSH_AUTH_SOCK, not user-supplied network input
}
