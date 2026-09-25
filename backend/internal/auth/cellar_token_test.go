package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The package's TestMain (e2e.Run) provides a local signing key.
func testPublicKey(t *testing.T) *rsa.PublicKey {
	t.Helper()
	pub, err := PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestCellarToken_RoundTrip(t *testing.T) {
	tok, err := SignCellarToken(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCellarToken(tok, testPublicKey(t)); err != nil {
		t.Fatal(err)
	}
}

func TestCellarToken_RejectsExpired(t *testing.T) {
	tok, err := SignCellarToken(-time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCellarToken(tok, testPublicKey(t)); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestCellarToken_RejectsOtherKey(t *testing.T) {
	tok, err := SignCellarToken(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCellarToken(tok, &other.PublicKey); err == nil {
		t.Fatal("token verified with a key that did not sign it")
	}
}

func TestCellarToken_AudiencesDoNotCross(t *testing.T) {
	userTok, err := CreateJWT(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCellarToken(userTok, testPublicKey(t)); err == nil {
		t.Fatal("user token accepted as a cellar token")
	}
	cellarTok, err := SignCellarToken(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateJWT(cellarTok); err == nil {
		t.Fatal("cellar token accepted as a user token")
	}
}
