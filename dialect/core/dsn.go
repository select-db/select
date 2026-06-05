package core

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Single DSN parser for all dialects. SSRF guard, SSH tunnel rewrite and
// secret masking share it so the validated host == the dialed host.

func pgLike(dbType string) bool { return strings.Contains(strings.ToLower(dbType), "postgre") }
func myLike(dbType string) bool { return strings.Contains(strings.ToLower(dbType), "mysql") }

func pgIsURL(dsn string) bool {
	t := strings.TrimSpace(dsn)
	return strings.HasPrefix(t, "postgres://") || strings.HasPrefix(t, "postgresql://")
}

// --- libpq key=value parsing (single-quote + backslash aware, like lib/pq) ---

type kvToken struct {
	key   string
	value string
}

// pgParseKV tokenises a libpq key=value DSN like lib/pq's parseOpts; errors
// on malformed input so the SSRF guard can fail closed.
func pgParseKV(dsn string) ([]kvToken, error) {
	var tokens []kvToken
	s := dsn
	i, n := 0, len(s)
	isSpace := func(b byte) bool {
		return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
	}
	for i < n {
		for i < n && isSpace(s[i]) {
			i++
		}
		if i >= n {
			break
		}
		keyStart := i
		for i < n && !isSpace(s[i]) && s[i] != '=' {
			i++
		}
		key := s[keyStart:i]
		for i < n && isSpace(s[i]) {
			i++
		}
		if i >= n || s[i] != '=' {
			return nil, fmt.Errorf("missing %q after %q in DSN", "=", key)
		}
		i++ // consume '='
		for i < n && isSpace(s[i]) {
			i++
		}
		var val strings.Builder
		if i < n && s[i] == '\'' {
			i++
			for {
				if i >= n {
					return nil, fmt.Errorf("unterminated quoted value in DSN")
				}
				c := s[i]
				i++
				if c == '\'' {
					break
				}
				if c == '\\' {
					if i >= n {
						return nil, fmt.Errorf("missing character after backslash in DSN")
					}
					c = s[i]
					i++
				}
				val.WriteByte(c)
			}
		} else {
			for i < n && !isSpace(s[i]) {
				c := s[i]
				i++
				if c == '\\' {
					if i >= n {
						return nil, fmt.Errorf("missing character after backslash in DSN")
					}
					c = s[i]
					i++
				}
				val.WriteByte(c)
			}
		}
		tokens = append(tokens, kvToken{key: key, value: val.String()})
	}
	return tokens, nil
}

func pgQuoteKV(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " \t\n\r\v\f'\\") {
		return v
	}
	r := strings.ReplaceAll(v, "\\", "\\\\")
	r = strings.ReplaceAll(r, "'", "\\'")
	return "'" + r + "'"
}

func pgBuildKV(tokens []kvToken) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, t.key+"="+pgQuoteKV(t.value))
	}
	return strings.Join(parts, " ")
}

// --- postgres URL (byte-level: net/url rejects/encodes non-ASCII userinfo) ---

type pgURL struct {
	prefix   string // up to and including "://"
	user     string
	pass     string
	hasUser  bool
	hasPass  bool
	hostport string // raw, may be "[ipv6]:port" or "host:port"
	tail     string // path/query/fragment, with its leading delimiter, or ""
}

func parsePGURL(dsn string) (pgURL, bool) {
	t := strings.TrimSpace(dsn)
	si := strings.Index(t, "://")
	if si < 0 {
		return pgURL{}, false
	}
	authStart := si + len("://")
	authEnd := len(t)
	for i := authStart; i < len(t); i++ {
		if c := t[i]; c == '/' || c == '?' || c == '#' {
			authEnd = i
			break
		}
	}
	u := pgURL{prefix: t[:authStart], tail: t[authEnd:]}
	authority := t[authStart:authEnd]
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		u.hasUser = true
		userinfo := authority[:at]
		u.hostport = authority[at+1:]
		if c := strings.Index(userinfo, ":"); c >= 0 {
			u.user = userinfo[:c]
			u.pass = userinfo[c+1:]
			u.hasPass = true
		} else {
			u.user = userinfo
		}
	} else {
		u.hostport = authority
	}
	return u, true
}

func (u pgURL) rebuild() string {
	var b strings.Builder
	b.WriteString(u.prefix)
	if u.hasUser {
		b.WriteString(u.user)
		if u.hasPass {
			b.WriteByte(':')
			b.WriteString(u.pass)
		}
		b.WriteByte('@')
	}
	b.WriteString(u.hostport)
	b.WriteString(u.tail)
	return b.String()
}

func (u pgURL) hostPort() (string, string) {
	hp := u.hostport
	if strings.HasPrefix(hp, "[") {
		if end := strings.IndexByte(hp, ']'); end >= 0 {
			host := hp[1:end]
			rest := hp[end+1:]
			if strings.HasPrefix(rest, ":") {
				return host, rest[1:]
			}
			return host, ""
		}
	}
	if c := strings.LastIndexByte(hp, ':'); c >= 0 {
		return hp[:c], hp[c+1:]
	}
	return hp, ""
}

// --- mysql go-sql-driver DSN: [user[:password]@][net[(addr)]]/db[?params] ---

func mysqlSplitCreds(dsn string) (user, pw, rest string, hasPw bool) {
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

const mysqlAddrMarker = "@tcp("

func mysqlAddrBounds(dsn string) (start, end int, ok bool) {
	m := strings.Index(dsn, mysqlAddrMarker)
	if m < 0 {
		return 0, 0, false
	}
	s := m + len(mysqlAddrMarker)
	e := strings.IndexByte(dsn[s:], ')')
	if e < 0 {
		return 0, 0, false
	}
	return s, s + e, true
}

// MySQLConnParams extracts user/pass/host/port/dbname from a go-sql-driver
// DSN: user:pass@tcp(host:port)/dbname[?params].
func MySQLConnParams(dsn string) (user, pass, host, port, dbname string, ok bool) {
	user, pass, _, _ = mysqlSplitCreds(dsn)
	s, e, found := mysqlAddrBounds(dsn)
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

// PostgresConnParams splits a postgres URL or libpq key=value DSN into
// components (defaults host 127.0.0.1, port 5432; ok=false without dbname).
// Lets pg_dump take discrete args instead of -d <dsn>, which would honour
// sslkey=/passfile=/service=/options=.
func PostgresConnParams(dsn string) (user, pass, host, port, dbname string, ok bool) {
	if pgIsURL(dsn) {
		u, parsed := parsePGURL(dsn)
		if !parsed {
			return "", "", "", "", "", false
		}
		user = u.user
		if u.hasPass {
			pass = u.pass
		}
		host, port = u.hostPort()
		t := u.tail
		if i := strings.IndexAny(t, "?#"); i >= 0 {
			t = t[:i]
		}
		dbname = strings.TrimPrefix(t, "/")
	} else {
		tokens, err := pgParseKV(dsn)
		if err != nil {
			return "", "", "", "", "", false
		}
		for _, tk := range tokens {
			switch tk.key {
			case "user":
				user = tk.value
			case "password":
				pass = tk.value
			case "host":
				host = tk.value
			case "port":
				port = tk.value
			case "dbname":
				dbname = tk.value
			}
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	ok = dbname != ""
	return
}

// --- exported high-level API (used by conn_cache, ssh tunnel, and backend) ---

// ParseDSNRemote returns the host:port the driver will dial. PostgreSQL URL
// and libpq key=value, MySQL @tcp().
func ParseDSNRemote(dbType, dsn string) (string, int, error) {
	switch {
	case pgLike(dbType):
		var host, portStr string
		if pgIsURL(dsn) {
			u, ok := parsePGURL(dsn)
			if !ok {
				return "", 0, fmt.Errorf("parse postgres URL")
			}
			host, portStr = u.hostPort()
		} else {
			tokens, err := pgParseKV(dsn)
			if err != nil {
				return "", 0, fmt.Errorf("parse postgres DSN: %w", err)
			}
			for _, t := range tokens {
				switch t.key {
				case "host":
					host = t.value
				case "port":
					portStr = t.value
				}
			}
		}
		if host == "" {
			host = "127.0.0.1"
		}
		if portStr == "" {
			portStr = "5432"
		}
		port, err := net.LookupPort("tcp", portStr)
		if err != nil {
			return "", 0, fmt.Errorf("parse postgres port %q: %w", portStr, err)
		}
		return host, port, nil

	case myLike(dbType):
		s, e, ok := mysqlAddrBounds(dsn)
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

	default:
		return "", 0, fmt.Errorf("ssh tunneling not supported for db type %s", dbType)
	}
}

// RewriteDSNForLocal repoints the DSN to localHost:localPort (post SSH tunnel).
func RewriteDSNForLocal(dbType, dsn, localHost string, localPort int) (string, error) {
	switch {
	case pgLike(dbType):
		if pgIsURL(dsn) {
			u, ok := parsePGURL(dsn)
			if !ok {
				return "", fmt.Errorf("parse postgres URL")
			}
			u.hostport = net.JoinHostPort(localHost, strconv.Itoa(localPort))
			return u.rebuild(), nil
		}
		tokens, err := pgParseKV(dsn)
		if err != nil {
			return "", fmt.Errorf("parse postgres DSN: %w", err)
		}
		var seenHost, seenPort bool
		for i := range tokens {
			switch tokens[i].key {
			case "host":
				tokens[i].value = localHost
				seenHost = true
			case "port":
				tokens[i].value = strconv.Itoa(localPort)
				seenPort = true
			}
		}
		if !seenHost {
			tokens = append(tokens, kvToken{"host", localHost})
		}
		if !seenPort {
			tokens = append(tokens, kvToken{"port", strconv.Itoa(localPort)})
		}
		return pgBuildKV(tokens), nil

	case myLike(dbType):
		s, e, ok := mysqlAddrBounds(dsn)
		if !ok {
			return "", fmt.Errorf("mysql dsn missing @tcp(...) segment")
		}
		return dsn[:s] + net.JoinHostPort(localHost, strconv.Itoa(localPort)) + dsn[e:], nil

	default:
		return "", fmt.Errorf("ssh tunneling not supported for db type %s", dbType)
	}
}

// DSNPassword returns the password embedded in the DSN, or "" if none or the
// DSN cannot be parsed.
func DSNPassword(dbType, dsn string) string {
	switch {
	case pgLike(dbType):
		if pgIsURL(dsn) {
			if u, ok := parsePGURL(dsn); ok && u.hasPass {
				return u.pass
			}
			return ""
		}
		tokens, err := pgParseKV(dsn)
		if err != nil {
			return ""
		}
		for _, t := range tokens {
			if t.key == "password" {
				return t.value
			}
		}
		return ""
	case myLike(dbType):
		if _, pw, _, hasPw := mysqlSplitCreds(dsn); hasPw {
			return pw
		}
		return ""
	default:
		return ""
	}
}

// DSNStripPassword removes the password component from the DSN.
func DSNStripPassword(dbType, dsn string) string {
	switch {
	case pgLike(dbType):
		if pgIsURL(dsn) {
			u, ok := parsePGURL(dsn)
			if !ok || !u.hasPass {
				return dsn
			}
			u.hasPass = false
			u.pass = ""
			return u.rebuild()
		}
		tokens, err := pgParseKV(dsn)
		if err != nil {
			return dsn
		}
		out := tokens[:0]
		for _, t := range tokens {
			if t.key == "password" {
				continue
			}
			out = append(out, t)
		}
		return pgBuildKV(out)
	case myLike(dbType):
		user, _, rest, hasPw := mysqlSplitCreds(dsn)
		if !hasPw {
			return dsn
		}
		return user + rest
	default:
		return dsn
	}
}

// DSNSetPassword injects pw as the password component of the DSN.
func DSNSetPassword(dbType, dsn, pw string) string {
	switch {
	case pgLike(dbType):
		if pgIsURL(dsn) {
			u, ok := parsePGURL(dsn)
			if !ok || !u.hasPass {
				return dsn
			}
			u.pass = pw
			return u.rebuild()
		}
		tokens, err := pgParseKV(dsn)
		if err != nil {
			return dsn
		}
		for i := range tokens {
			if tokens[i].key == "password" {
				tokens[i].value = pw
				return pgBuildKV(tokens)
			}
		}
		tokens = append(tokens, kvToken{"password", pw})
		return pgBuildKV(tokens)
	case myLike(dbType):
		user, _, rest, _ := mysqlSplitCreds(dsn)
		return user + ":" + pw + rest
	default:
		return dsn
	}
}
