package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestJWTRoundTripLocalSigner proves the crypto.Signer path: a local RSA key
// signs via the custom rsaSignerMethod and the resulting token verifies with
// the stock RS256 verifier (signature must be a standard PKCS#1 v1.5 RS256 sig).
func TestJWTRoundTripLocalSigner(t *testing.T) {
	userID := uuid.New()
	tok, err := CreateJWT(context.Background(), userID)
	if err != nil {
		t.Fatalf("CreateJWT: %v", err)
	}

	_, claims, err := ValidateJWT(tok)
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if claims.UserID != userID.String() {
		t.Fatalf("sub mismatch: got %q want %q", claims.UserID, userID.String())
	}
}

// TestJWTRejectsTampered confirms a flipped payload fails verification.
func TestJWTRejectsTampered(t *testing.T) {
	tok, err := CreateJWT(context.Background(), uuid.New())
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

func TestVerifyRejects(t *testing.T) {
	pub, err := PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := Sign(CustomClaims{}, "svc", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := Sign(CustomClaims{}, "svc", -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Verify(valid, pub, "svc"); err != nil {
		t.Fatalf("valid token refused: %v", err)
	}
	for name, check := range map[string]func() error{
		"expired":        func() error { _, _, err := Verify(expired, pub, "svc"); return err },
		"other key":      func() error { _, _, err := Verify(valid, &other.PublicKey, "svc"); return err },
		"other audience": func() error { _, _, err := ValidateJWT(valid); return err },
	} {
		if check() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
