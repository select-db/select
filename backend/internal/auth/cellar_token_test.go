package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// useLocalSigner points the JWT signer at a fresh local key. The signer loads
// once per process, so every test in the package shares the first key.
func useLocalSigner(t *testing.T) *rsa.PublicKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	path := filepath.Join(t.TempDir(), "jwt.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SELECTDB_KEK", "dev")
	t.Setenv("PRIVATE_KEY_PATH", path)
	pub, err := PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestCellarToken_RoundTrip(t *testing.T) {
	pub := useLocalSigner(t)
	want := CellarClaims{DB: "db-1", WS: "ws-1", Cel: "local", Max: 250 << 20, PITR: 1, Perm: "abc"}
	tok, err := SignCellarToken(want, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ValidateCellarToken(tok, pub)
	if err != nil {
		t.Fatal(err)
	}
	if got.DB != want.DB || got.WS != want.WS || got.Cel != want.Cel || got.Max != want.Max || got.PITR != want.PITR || got.Perm != want.Perm {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCellarToken_RejectsExpired(t *testing.T) {
	pub := useLocalSigner(t)
	tok, err := SignCellarToken(CellarClaims{DB: "db-1"}, -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateCellarToken(tok, pub); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestCellarToken_RejectsOtherKey(t *testing.T) {
	useLocalSigner(t)
	tok, err := SignCellarToken(CellarClaims{DB: "db-1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateCellarToken(tok, &other.PublicKey); err == nil {
		t.Fatal("token verified with a key that did not sign it")
	}
}

func TestCellarToken_AudiencesDoNotCross(t *testing.T) {
	pub := useLocalSigner(t)
	userTok, err := CreateJWT(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateCellarToken(userTok, pub); err == nil {
		t.Fatal("user token accepted as a cellar token")
	}
	cellarTok, err := SignCellarToken(CellarClaims{DB: "db-1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateJWT(cellarTok); err == nil {
		t.Fatal("cellar token accepted as a user token")
	}
}
