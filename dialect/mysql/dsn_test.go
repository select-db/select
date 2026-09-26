package mysql

import "testing"

func TestDSNPasswordRoundTrip(t *testing.T) {
	d := NewDialect()
	dsn := "root:tiger@tcp(127.0.0.1:3306)/app"
	if got := d.DSNPassword(dsn); got != "tiger" {
		t.Fatalf("DSNPassword = %q, want tiger", got)
	}
	if stripped := d.DSNWithoutPassword(dsn); d.DSNPassword(stripped) != "" {
		t.Fatalf("DSNWithoutPassword left a password: %q", stripped)
	}
	if reset := d.DSNWithPassword(dsn, "NEWPW"); d.DSNPassword(reset) != "NEWPW" {
		t.Fatalf("DSNWithPassword failed: %q", reset)
	}
}

func TestDSNHost(t *testing.T) {
	h, p, err := NewDialect().DSNHost("root:pw@tcp(10.0.0.5:3307)/app?parseTime=true")
	if err != nil || h != "10.0.0.5" || p != 3307 {
		t.Fatalf("DSNHost = %q,%d,%v want 10.0.0.5,3307", h, p, err)
	}
}

func TestDSNWithHost(t *testing.T) {
	got, err := NewDialect().DSNWithHost("root:p@tcp(remote:3306)/app?parseTime=true", "127.0.0.1", 7777)
	if want := "root:p@tcp(127.0.0.1:7777)/app?parseTime=true"; err != nil || got != want {
		t.Fatalf("DSNWithHost = %q,%v want %q", got, err, want)
	}
}
