//go:build windows

package engine

import (
	"io"
	"net"
	"os"
	"strings"
)

// windowsOpenSSHAgentPipe is the named pipe the Windows OpenSSH agent listens on.
const windowsOpenSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent connects to the SSH agent on Windows. The OpenSSH agent exposes a
// named pipe rather than a unix socket, and SSH_AUTH_SOCK is usually unset.
// SSH_AUTH_SOCK is honored when present (a pipe path, or a unix-socket path under
// WSL / Git-Bash); otherwise the default OpenSSH agent pipe is used.
func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		sock = windowsOpenSSHAgentPipe
	}
	if strings.HasPrefix(sock, `\\`) {
		// Named pipe: open as a file (CreateFile under the hood). The returned
		// *os.File satisfies io.ReadWriteCloser for the agent protocol.
		return os.OpenFile(sock, os.O_RDWR, 0) // #nosec G304 -- the user's own agent pipe, not user-supplied input
	}
	// Unix-socket path (AF_UNIX is supported on Windows 10+).
	return net.Dial("unix", sock) // #nosec G704 -- local agent socket from SSH_AUTH_SOCK, not user-supplied network input
}
