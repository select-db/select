package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/db/db_types"

	"github.com/google/uuid"
)

// TestJWTRoundTripLocalSigner proves the crypto.Signer path: a local RSA key
// signs via the custom rsaSignerMethod and the resulting token verifies with
// the stock RS256 verifier (signature must be a standard PKCS#1 v1.5 RS256 sig).
func TestJWTRoundTripLocalSigner(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "jwt.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SELECTDB_KEK", "dev") // force localMode -> in-process signer
	t.Setenv("PRIVATE_KEY_PATH", path)

	userID := db_types.NewJSONNullUUID(uuid.New())
	tok, err := CreateJWT(context.Background(), userID)
	if err != nil {
		t.Fatalf("CreateJWT: %v", err)
	}

	_, claims, err := ValidateJWT(tok)
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if claims.UserID != userID.UUID.String() {
		t.Fatalf("sub mismatch: got %q want %q", claims.UserID, userID.UUID.String())
	}
}

// TestJWTRejectsTampered confirms a flipped payload fails verification.
func TestJWTRejectsTampered(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "jwt.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SELECTDB_KEK", "dev")
	t.Setenv("PRIVATE_KEY_PATH", path)

	tok, err := CreateJWT(context.Background(), db_types.NewJSONNullUUID(uuid.New()))
	if err != nil {
		t.Fatal(err)
	}
	// Flip the first char of the payload segment. It's fully significant (byte 0
	// of the payload), so the signed message always changes and verification
	// always fails — unlike mutating the last signature char, whose low bits
	// base64 ignores, which passes ~1/256 of the time when the decoded byte is
	// unchanged.
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape: %d parts", len(parts))
	}
	p := []byte(parts[1])
	if p[0] == 'A' {
		p[0] = 'B'
	} else {
		p[0] = 'A'
	}
	tampered := parts[0] + "." + string(p) + "." + parts[2]
	if _, _, err := ValidateJWT(tampered); err == nil {
		t.Fatal("expected verification failure on tampered token")
	}
}
