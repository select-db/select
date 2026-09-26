package mysql

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// A go-sql-driver DSN: [user[:password]@][net[(addr)]]/db[?params].

// DSNHost returns the host and port in the DSN's @tcp(...) address.
func (d *Dialect) DSNHost(dsn string) (string, int, error) {
	s, e, ok := addrBounds(dsn)
	if !ok {
		return "", 0, fmt.Errorf("mysql dsn missing @tcp(...) segment")
	}
	host, portStr, err := net.SplitHostPort(dsn[s:e])
	if err != nil {
		host = dsn[s:e]
		portStr = "3306"
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parse mysql port %q: %w", portStr, err)
	}
	return host, port, nil
}

// DSNWithHost returns dsn pointed at host:port.
func (d *Dialect) DSNWithHost(dsn, host string, port int) (string, error) {
	s, e, ok := addrBounds(dsn)
	if !ok {
		return "", fmt.Errorf("mysql dsn missing @tcp(...) segment")
	}
	return dsn[:s] + net.JoinHostPort(host, strconv.Itoa(port)) + dsn[e:], nil
}

// DSNPassword returns the password in dsn, or "" when it holds none.
func (d *Dialect) DSNPassword(dsn string) string {
	_, pw, _, _ := splitCreds(dsn)
	return pw
}

// DSNWithPassword returns dsn holding password.
func (d *Dialect) DSNWithPassword(dsn, password string) string {
	user, _, rest, _ := splitCreds(dsn)
	return user + ":" + password + rest
}

// DSNWithoutPassword returns dsn with no password.
func (d *Dialect) DSNWithoutPassword(dsn string) string {
	user, _, rest, hasPw := splitCreds(dsn)
	if !hasPw {
		return dsn
	}
	return user + rest
}

func splitCreds(dsn string) (user, pw, rest string, hasPw bool) {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return "", "", dsn, false
	}
	cred := dsn[:at]
	rest = dsn[at:] // keeps leading '@'
	if i := strings.IndexByte(cred, ':'); i >= 0 {
		return cred[:i], cred[i+1:], rest, true
	}
	return cred, "", rest, false
}

const addrMarker = "@tcp("

func addrBounds(dsn string) (start, end int, ok bool) {
	m := strings.Index(dsn, addrMarker)
	if m < 0 {
		return 0, 0, false
	}
	s := m + len(addrMarker)
	e := strings.IndexByte(dsn[s:], ')')
	if e < 0 {
		return 0, 0, false
	}
	return s, s + e, true
}

// connParams extracts user/pass/host/port/dbname from a go-sql-driver
// DSN: user:pass@tcp(host:port)/dbname[?params].
func connParams(dsn string) (user, pass, host, port, dbname string, ok bool) {
	user, pass, _, _ = splitCreds(dsn)
	s, e, found := addrBounds(dsn)
	if !found {
		return "", "", "", "", "", false
	}
	addr := dsn[s:e]
	if c := strings.LastIndexByte(addr, ':'); c >= 0 {
		host, port = addr[:c], addr[c+1:]
	} else {
		host, port = addr, "3306"
	}
	after := strings.TrimPrefix(dsn[e+1:], "/")
	if q := strings.IndexByte(after, '?'); q >= 0 {
		after = after[:q]
	}
	dbname = after
	ok = user != "" && host != "" && dbname != ""
	return
}
