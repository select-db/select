package mysql

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// A go-sql-driver DSN is [user[:password]@][net[(addr)]]/db[?params].

var errNoTCPAddr = errors.New("mysql dsn missing @tcp(...) segment")

// DSNHost returns the host and port in the DSN's @tcp(...) address.
func (d *Dialect) DSNHost(dsn string) (string, int, error) {
	start, end, ok := addrBounds(dsn)
	if !ok {
		return "", 0, errNoTCPAddr
	}
	host, portStr, err := net.SplitHostPort(dsn[start:end])
	if err != nil {
		host = dsn[start:end]
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
	start, end, ok := addrBounds(dsn)
	if !ok {
		return "", errNoTCPAddr
	}
	return dsn[:start] + net.JoinHostPort(host, strconv.Itoa(port)) + dsn[end:], nil
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
	start = m + len(addrMarker)
	n := strings.IndexByte(dsn[start:], ')')
	if n < 0 {
		return 0, 0, false
	}
	return start, start + n, true
}

// connParams returns the parts of dsn mysqldump takes as arguments. ok is
// false without a user, a host or a dbname.
func (d *Dialect) connParams(dsn string) (user, pass, host, port, dbname string, ok bool) {
	h, p, err := d.DSNHost(dsn)
	if err != nil {
		return "", "", "", "", "", false
	}
	host, port = h, strconv.Itoa(p)
	user, pass, _, _ = splitCreds(dsn)
	_, end, _ := addrBounds(dsn)
	dbname = strings.TrimPrefix(dsn[end+1:], "/")
	if q := strings.IndexByte(dbname, '?'); q >= 0 {
		dbname = dbname[:q]
	}
	return user, pass, host, port, dbname, user != "" && host != "" && dbname != ""
}
