package connect

import (
	"fmt"

	"github.com/selectDb/dialect/sqlite"
)

// ResolveDumpDSN returns a DSN safe for the out-of-process dump tools, which
// resolve/dial the host themselves (the Go guarded dialer can't reach them).
// Pins the target like the driver path.
func ResolveDumpDSN(workspaceID, dbType, dsn string, ssh *ResolvedSSHConfig) (string, error) {
	dialect, err := dialectFor(dbType)
	if err != nil {
		return "", err
	}

	// Rewritten to the tunnel's local endpoint:
	//   - any datasource with SSH
	// Refused:
	//   - a DSN with no host to tunnel to, such as a sqlite file
	if ssh != nil {
		return tunneledDSN(workspaceID, dialect, dsn, *ssh)
	}

	// Unchanged:
	//   - the desktop app
	//   - a cellar DSN, whose driver dials only the configured cellar
	if !EnforceOutboundGuard || sqlite.IsCellarDSN(dsn) {
		return dsn, nil
	}

	// Rewritten to the resolved and validated literal IP, so the tool cannot re-resolve:
	//   - the server dialing a user's DSN directly
	// Refused:
	//   - a DSN whose host does not parse, incl. a sqlite file (a path on this host)
	host, port, err := dialect.DSNHost(dsn)
	if err != nil {
		return "", fmt.Errorf("connection target is not permitted")
	}
	ips, err := resolveAllowed(host, isBlockedIP)
	if err != nil {
		return "", err
	}
	return dialect.DSNWithHost(dsn, ips[0].String(), port)
}
