package cellar

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokens_ReusesUntilSomethingDiffers(t *testing.T) {
	signed := 0
	tk := NewTokens()
	tk.sign = func(c auth.CellarClaims, _ time.Duration) (string, error) {
		signed++
		return c.DB + "/" + c.Perm, nil
	}
	c := auth.CellarClaims{DB: "db-1", WS: "ws-1", Cel: "local", Perm: "p1"}
	for range 3 {
		if _, err := tk.Token(c); err != nil {
			t.Fatal(err)
		}
	}
	if signed != 1 {
		t.Fatalf("signed %d times for identical claims, want 1", signed)
	}
	c.Perm = "p2"
	if tok, _ := tk.Token(c); tok != "db-1/p2" || signed != 2 {
		t.Fatalf("changed permissions reused a token: %q after %d signs", tok, signed)
	}
}

// signWith signs claims with priv, standing in for the backend's KMS signer.
func signWith(t *testing.T, priv *rsa.PrivateKey, c auth.CellarClaims) string {
	t.Helper()
	c.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    auth.Issuer,
		Audience:  jwt.ClaimStrings{auth.CellarAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestAuthorize(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	perms := []byte(`[{"Action":"select","Effect":"allow"}]`)
	good := auth.CellarClaims{DB: "db-1", WS: "ws-1", Cel: "local", Perm: PermHash(perms)}
	userTok := func() string {
		c := jwt.RegisteredClaims{Issuer: auth.Issuer, Audience: jwt.ClaimStrings{auth.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(priv)
		return tok
	}()
	wrongDB, wrongCel := good, good
	wrongDB.DB, wrongCel.Cel = "db-2", "cellar-2"

	cases := []struct {
		name   string
		token  string
		perms  []byte
		wantOK bool
	}{
		{"valid", signWith(t, priv, good), perms, true},
		{"no token", "", perms, false},
		{"user token", userTok, perms, false},
		{"other db", signWith(t, priv, wrongDB), perms, false},
		{"other cellar", signWith(t, priv, wrongCel), perms, false},
		{"widened permissions", signWith(t, priv, good), []byte(`[{"Action":"manage","Effect":"allow"}]`), false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/x", nil)
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		_, err := Authorize(r, &priv.PublicKey, "local", "db-1", c.perms)
		if (err == nil) != c.wantOK {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.wantOK)
		}
	}
}
