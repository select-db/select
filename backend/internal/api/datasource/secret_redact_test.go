package datasource

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func genTestKey(t *testing.T) (pemStr, wantFingerprint string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemStr = string(pem.EncodeToMemory(block))
	signer, err := ssh.ParsePrivateKey([]byte(pemStr))
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return pemStr, ssh.FingerprintSHA256(signer.PublicKey())
}

func TestPreviewPassword(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"short":                     maskCore,               // n<8 fully masked
		"hunter22":                  "h" + maskCore + "2",   // n=8  -> 1+core+1
		"hunter2025":                "h" + maskCore + "5",   // n=10 -> 1+core+1
		"supersecret1":              "su" + maskCore + "t1", // n=12 -> 2+core+2
		"correcthorsebatterystaple": "co" + maskCore + "le",
	}
	for in, want := range cases {
		if got := previewPassword(in); got != want {
			t.Errorf("previewPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPreviewKey(t *testing.T) {
	if got := previewKey(""); got != "" {
		t.Errorf("previewKey(empty) = %q, want empty", got)
	}

	pemStr, fp := genTestKey(t)
	got := previewKey(pemStr)
	want := fp + " (ED25519)"
	if got != want {
		t.Errorf("previewKey = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "SHA256:") {
		t.Errorf("previewKey = %q, want OpenSSH SHA256 fingerprint", got)
	}
	if got != previewKey(pemStr) {
		t.Error("previewKey must be deterministic")
	}

	// Unparseable input still yields a stable, non-leaking identifier.
	junk := "-----BEGIN OPENSSH PRIVATE KEY-----\nNOTAREALKEY\n-----END OPENSSH PRIVATE KEY-----"
	if g := previewKey(junk); !strings.HasPrefix(g, "private key · ") || g != previewKey(junk) {
		t.Errorf("previewKey(junk) = %q, want stable 'private key · <hash>'", g)
	}
}

func TestMaskAndMergeDSN(t *testing.T) {
	pw := "s3cr3tPlus12" // len 12 -> s3••••12
	prev := previewPassword(pw)

	cases := []struct {
		name       string
		dbType     string
		stored     string
		wantMask   string
		clientBack string
		wantStored string
	}{
		{
			name:       "postgres url untouched keeps password",
			dbType:     "postgresql",
			stored:     "postgres://alice:" + pw + "@db.example.com:5432/app?sslmode=require",
			wantMask:   "postgres://alice:" + prev + "@db.example.com:5432/app?sslmode=require",
			clientBack: "postgres://alice:" + prev + "@db.example.com:5432/app?sslmode=require",
			wantStored: "postgres://alice:" + pw + "@db.example.com:5432/app?sslmode=require",
		},
		{
			name:       "postgres url edited host keeps password",
			dbType:     "postgresql",
			stored:     "postgres://alice:" + pw + "@db1/app",
			wantMask:   "postgres://alice:" + prev + "@db1/app",
			clientBack: "postgres://alice:" + prev + "@db2/app",
			wantStored: "postgres://alice:" + pw + "@db2/app",
		},
		{
			name:       "postgres url new password stored verbatim",
			dbType:     "postgresql",
			stored:     "postgres://alice:" + pw + "@db1/app",
			wantMask:   "postgres://alice:" + prev + "@db1/app",
			clientBack: "postgres://alice:brandnewpass@db1/app",
			wantStored: "postgres://alice:brandnewpass@db1/app",
		},
		{
			name:       "mysql untouched keeps password",
			dbType:     "mysql",
			stored:     "root:" + pw + "@tcp(127.0.0.1:3306)/app",
			wantMask:   "root:" + prev + "@tcp(127.0.0.1:3306)/app",
			clientBack: "root:" + prev + "@tcp(127.0.0.1:3306)/app",
			wantStored: "root:" + pw + "@tcp(127.0.0.1:3306)/app",
		},
		{
			name:       "sqlite has no password",
			dbType:     "sqlite",
			stored:     "file:/data/app.db?cache=shared",
			wantMask:   "file:/data/app.db?cache=shared",
			clientBack: "file:/data/app.db?cache=shared",
			wantStored: "file:/data/app.db?cache=shared",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maskDSN(c.dbType, c.stored); got != c.wantMask {
				t.Errorf("maskDSN = %q, want %q", got, c.wantMask)
			}
			if got := mergeDSN(c.dbType, c.clientBack, c.stored); got != c.wantStored {
				t.Errorf("mergeDSN = %q, want %q", got, c.wantStored)
			}
		})
	}
}

func TestMaskAndMergeSSH(t *testing.T) {
	pw := "hunterPW2025"
	keyPEM, _ := genTestKey(t)
	keyJSON := strings.ReplaceAll(keyPEM, "\n", "\\n")
	stored := `{"enabled":true,"host":"bastion","port":22,"user":"deploy","auth_method":"password","password":"` + pw + `","private_key":"` + keyJSON + `"}`

	masked := maskSSH(stored)
	if strings.Contains(masked, pw) || strings.Contains(masked, "PRIVATE KEY-----") {
		t.Fatalf("masked SSH leaks a secret: %s", masked)
	}
	if !strings.Contains(masked, previewPassword(pw)) || !strings.Contains(masked, "SHA256:") {
		t.Fatalf("masked SSH missing previews: %s", masked)
	}

	// Untouched: client sends the masked previews back -> stored kept.
	merged := mergeSSH(masked, stored)
	if !strings.Contains(merged, pw) || !strings.Contains(merged, "PRIVATE KEY-----") {
		t.Fatalf("mergeSSH dropped stored secrets: %s", merged)
	}

	// New password typed, key preview left untouched.
	edited := `{"password":"freshpassword","private_key":"` + previewKey(strings.ReplaceAll(keyJSON, "\\n", "\n")) + `"}`
	merged2 := mergeSSH(edited, stored)
	if !strings.Contains(merged2, "freshpassword") || !strings.Contains(merged2, "PRIVATE KEY-----") {
		t.Fatalf("mergeSSH should keep new pw and stored key: %s", merged2)
	}

	// Password cleared -> stored secret removed.
	cleared := `{"password":"","private_key":"` + previewKey(strings.ReplaceAll(keyJSON, "\\n", "\n")) + `"}`
	if strings.Contains(mergeSSH(cleared, stored), pw) {
		t.Fatalf("cleared password should not be kept")
	}
}
