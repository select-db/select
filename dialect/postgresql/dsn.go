package postgresql

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DSNHost returns the host and port the driver dials, from a URL or a libpq
// key=value DSN. It fails on a DSN it cannot parse, so the guard fails closed.
func (d *Dialect) DSNHost(dsn string) (string, int, error) {
	var host, portStr string
	if isURLDSN(dsn) {
		u, ok := parseURLDSN(dsn)
		if !ok {
			return "", 0, fmt.Errorf("parse postgres URL")
		}
		host, portStr = u.hostPort()
	} else {
		tokens, err := parseKV(dsn)
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
}

// DSNWithHost returns dsn pointed at host:port.
func (d *Dialect) DSNWithHost(dsn, host string, port int) (string, error) {
	if isURLDSN(dsn) {
		u, ok := parseURLDSN(dsn)
		if !ok {
			return "", fmt.Errorf("parse postgres URL")
		}
		u.hostport = net.JoinHostPort(host, strconv.Itoa(port))
		return u.rebuild(), nil
	}
	tokens, err := parseKV(dsn)
	if err != nil {
		return "", fmt.Errorf("parse postgres DSN: %w", err)
	}
	var seenHost, seenPort bool
	for i := range tokens {
		switch tokens[i].key {
		case "host":
			tokens[i].value = host
			seenHost = true
		case "port":
			tokens[i].value = strconv.Itoa(port)
			seenPort = true
		}
	}
	if !seenHost {
		tokens = append(tokens, kvToken{"host", host})
	}
	if !seenPort {
		tokens = append(tokens, kvToken{"port", strconv.Itoa(port)})
	}
	return buildKV(tokens), nil
}

// DSNPassword returns the password in dsn, or "" when it holds none or does
// not parse.
func (d *Dialect) DSNPassword(dsn string) string {
	if isURLDSN(dsn) {
		if u, ok := parseURLDSN(dsn); ok && u.hasPass {
			return u.pass
		}
		return ""
	}
	tokens, err := parseKV(dsn)
	if err != nil {
		return ""
	}
	for _, t := range tokens {
		if t.key == "password" {
			return t.value
		}
	}
	return ""
}

// DSNWithPassword returns dsn holding password. A URL without one is left as is.
func (d *Dialect) DSNWithPassword(dsn, password string) string {
	if isURLDSN(dsn) {
		u, ok := parseURLDSN(dsn)
		if !ok || !u.hasPass {
			return dsn
		}
		u.pass = password
		return u.rebuild()
	}
	tokens, err := parseKV(dsn)
	if err != nil {
		return dsn
	}
	for i := range tokens {
		if tokens[i].key == "password" {
			tokens[i].value = password
			return buildKV(tokens)
		}
	}
	return buildKV(append(tokens, kvToken{"password", password}))
}

// DSNWithoutPassword returns dsn with no password.
func (d *Dialect) DSNWithoutPassword(dsn string) string {
	if isURLDSN(dsn) {
		u, ok := parseURLDSN(dsn)
		if !ok || !u.hasPass {
			return dsn
		}
		u.hasPass = false
		u.pass = ""
		return u.rebuild()
	}
	tokens, err := parseKV(dsn)
	if err != nil {
		return dsn
	}
	out := tokens[:0]
	for _, t := range tokens {
		if t.key != "password" {
			out = append(out, t)
		}
	}
	return buildKV(out)
}

func isURLDSN(dsn string) bool {
	t := strings.TrimSpace(dsn)
	return strings.HasPrefix(t, "postgres://") || strings.HasPrefix(t, "postgresql://")
}

// libpq key=value parsing, single-quote and backslash aware like lib/pq.

type kvToken struct {
	key   string
	value string
}

// parseKV tokenises a libpq key=value DSN like lib/pq's parseOpts; errors
// on malformed input so the SSRF guard can fail closed.
func parseKV(dsn string) ([]kvToken, error) {
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

func quoteKV(v string) string {
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

func buildKV(tokens []kvToken) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, t.key+"="+quoteKV(t.value))
	}
	return strings.Join(parts, " ")
}

// A postgres URL is parsed byte by byte: net/url rejects or encodes non-ASCII userinfo.

type urlDSN struct {
	prefix   string // up to and including "://"
	user     string
	pass     string
	hasUser  bool
	hasPass  bool
	hostport string // raw, may be "[ipv6]:port" or "host:port"
	tail     string // path/query/fragment, with its leading delimiter, or ""
}

func parseURLDSN(dsn string) (urlDSN, bool) {
	t := strings.TrimSpace(dsn)
	si := strings.Index(t, "://")
	if si < 0 {
		return urlDSN{}, false
	}
	authStart := si + len("://")
	authEnd := len(t)
	for i := authStart; i < len(t); i++ {
		if c := t[i]; c == '/' || c == '?' || c == '#' {
			authEnd = i
			break
		}
	}
	u := urlDSN{prefix: t[:authStart], tail: t[authEnd:]}
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

func (u urlDSN) rebuild() string {
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

func (u urlDSN) hostPort() (string, string) {
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

// connParams splits a postgres URL or libpq key=value DSN into
// components (defaults host 127.0.0.1, port 5432; ok=false without dbname).
// Lets pg_dump take discrete args instead of -d <dsn>, which would honour
// sslkey=/passfile=/service=/options=.
func connParams(dsn string) (user, pass, host, port, dbname string, ok bool) {
	if isURLDSN(dsn) {
		u, parsed := parseURLDSN(dsn)
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
		tokens, err := parseKV(dsn)
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
