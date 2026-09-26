package postgresql

import "testing"

func TestParseKV(t *testing.T) {
	tokens, err := parseKV("host='169.254.169.254' port = 80 user=app password='p w'")
	if err != nil {
		t.Fatalf("parseKV error: %v", err)
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
		if _, err := parseKV(bad); err == nil {
			t.Errorf("parseKV(%q) = nil err, want error (fail closed)", bad)
		}
	}
}

func TestDSNPasswordRoundTrip(t *testing.T) {
	d := NewDialect()
	cases := []struct{ name, dsn, pw string }{
		{"url", "postgres://alice:s3cr3t@db:5432/app?sslmode=require", "s3cr3t"},
		{"url unicode pw", "postgres://alice:a••••b@db/app", "a••••b"},
		{"kv", "host=db user=bob password='p@ss w' dbname=app", "p@ss w"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := d.DSNPassword(c.dsn); got != c.pw {
				t.Fatalf("DSNPassword = %q, want %q", got, c.pw)
			}
			if stripped := d.DSNWithoutPassword(c.dsn); d.DSNPassword(stripped) != "" {
				t.Fatalf("DSNWithoutPassword left a password: %q", stripped)
			}
			if reset := d.DSNWithPassword(c.dsn, "NEWPW"); d.DSNPassword(reset) != "NEWPW" {
				t.Fatalf("DSNWithPassword failed: %q", reset)
			}
		})
	}
}

func TestDSNHost(t *testing.T) {
	d := NewDialect()
	cases := []struct {
		dsn, host string
		port      int
	}{
		{"postgres://u:p@db.example.com:6543/app", "db.example.com", 6543},
		{"postgres://u:p@[::1]:5433/app", "::1", 5433},
		{"host='10.0.0.9' port=5432 user=x", "10.0.0.9", 5432},
		{"user=x dbname=y", "127.0.0.1", 5432},
	}
	for _, c := range cases {
		h, p, err := d.DSNHost(c.dsn)
		if err != nil {
			t.Errorf("DSNHost(%q) error: %v", c.dsn, err)
			continue
		}
		if h != c.host || p != c.port {
			t.Errorf("DSNHost(%q) = %q,%d want %q,%d", c.dsn, h, p, c.host, c.port)
		}
	}

	if _, _, err := d.DSNHost("host='unterminated"); err == nil {
		t.Error("DSNHost should error on malformed DSN (fail closed)")
	}
}

func TestDSNWithHost(t *testing.T) {
	d := NewDialect()
	cases := []struct{ dsn, want string }{
		{
			"postgres://u:p@remote:5432/app?sslmode=require",
			"postgres://u:p@127.0.0.1:7777/app?sslmode=require",
		},
		{
			// a quoted value a whitespace split would corrupt
			"host=remote port=5432 password='a b' dbname=app",
			"host=127.0.0.1 port=7777 password='a b' dbname=app",
		},
	}
	for _, c := range cases {
		got, err := d.DSNWithHost(c.dsn, "127.0.0.1", 7777)
		if err != nil {
			t.Errorf("DSNWithHost(%q) error: %v", c.dsn, err)
			continue
		}
		if got != c.want {
			t.Errorf("DSNWithHost(%q) = %q, want %q", c.dsn, got, c.want)
		}
	}
}
