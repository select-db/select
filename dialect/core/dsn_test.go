package core

import "testing"

func TestPGParseKV(t *testing.T) {
	tokens, err := pgParseKV("host='169.254.169.254' port = 80 user=app password='p w'")
	if err != nil {
		t.Fatalf("pgParseKV error: %v", err)
	}
	got := map[string]string{}
	for _, tk := range tokens {
		got[tk.key] = tk.value
	}
	if got["host"] != "169.254.169.254" {
		t.Errorf("host = %q, want unquoted 169.254.169.254", got["host"])
	}
	if got["port"] != "80" || got["user"] != "app" || got["password"] != "p w" {
		t.Errorf("unexpected tokens: %#v", got)
	}

	for _, bad := range []string{"hostonly", "host='unterminated", `host=x\`} {
		if _, err := pgParseKV(bad); err == nil {
			t.Errorf("pgParseKV(%q) = nil err, want error (fail closed)", bad)
		}
	}
}

func TestDSNPasswordRoundTrip(t *testing.T) {
	cases := []struct{ name, dbType, dsn, pw string }{
		{"pg url", "postgresql", "postgres://alice:s3cr3t@db:5432/app?sslmode=require", "s3cr3t"},
		{"pg url unicode pw", "postgresql", "postgres://alice:a••••b@db/app", "a••••b"},
		{"pg kv", "postgresql", "host=db user=bob password='p@ss w' dbname=app", "p@ss w"},
		{"mysql", "mysql", "root:tiger@tcp(127.0.0.1:3306)/app", "tiger"},
		{"sqlite none", "sqlite", "file:/data/app.db", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DSNPassword(c.dbType, c.dsn); got != c.pw {
				t.Fatalf("DSNPassword = %q, want %q", got, c.pw)
			}
			if c.pw == "" {
				return
			}
			stripped := DSNStripPassword(c.dbType, c.dsn)
			if DSNPassword(c.dbType, stripped) != "" {
				t.Fatalf("DSNStripPassword left a password: %q", stripped)
			}
			reset := DSNSetPassword(c.dbType, c.dsn, "NEWPW")
			if DSNPassword(c.dbType, reset) != "NEWPW" {
				t.Fatalf("DSNSetPassword failed: %q", reset)
			}
		})
	}
}

func TestParseDSNRemote(t *testing.T) {
	cases := []struct {
		dbType, dsn, host string
		port              int
	}{
		{"postgresql", "postgres://u:p@db.example.com:6543/app", "db.example.com", 6543},
		{"postgresql", "postgres://u:p@[::1]:5433/app", "::1", 5433},
		{"postgresql", "host='10.0.0.9' port=5432 user=x", "10.0.0.9", 5432},
		{"postgresql", "user=x dbname=y", "127.0.0.1", 5432},
		{"mysql", "root:pw@tcp(10.0.0.5:3307)/app?parseTime=true", "10.0.0.5", 3307},
	}
	for _, c := range cases {
		h, p, err := ParseDSNRemote(c.dbType, c.dsn)
		if err != nil {
			t.Errorf("ParseDSNRemote(%q) error: %v", c.dsn, err)
			continue
		}
		if h != c.host || p != c.port {
			t.Errorf("ParseDSNRemote(%q) = %q,%d want %q,%d", c.dsn, h, p, c.host, c.port)
		}
	}

	if _, _, err := ParseDSNRemote("postgresql", "host='unterminated"); err == nil {
		t.Error("ParseDSNRemote should error on malformed DSN (fail closed)")
	}
}

func TestRewriteDSNForLocal(t *testing.T) {
	cases := []struct{ dbType, dsn, want string }{
		{
			"postgresql",
			"postgres://u:p@remote:5432/app?sslmode=require",
			"postgres://u:p@127.0.0.1:7777/app?sslmode=require",
		},
		{
			// quoted value the old strings.Fields parser corrupted
			"postgresql",
			"host=remote port=5432 password='a b' dbname=app",
			"host=127.0.0.1 port=7777 password='a b' dbname=app",
		},
		{
			"mysql",
			"root:p@tcp(remote:3306)/app?parseTime=true",
			"root:p@tcp(127.0.0.1:7777)/app?parseTime=true",
		},
	}
	for _, c := range cases {
		got, err := RewriteDSNForLocal(c.dbType, c.dsn, "127.0.0.1", 7777)
		if err != nil {
			t.Errorf("RewriteDSNForLocal(%q) error: %v", c.dsn, err)
			continue
		}
		if got != c.want {
			t.Errorf("RewriteDSNForLocal(%q) = %q, want %q", c.dsn, got, c.want)
		}
	}
}
