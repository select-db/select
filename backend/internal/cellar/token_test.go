package cellar

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokens_ReusesUntilTheGrantChanges(t *testing.T) {
	signed := 0
	tk := NewTokens()
	tk.sign = func(g auth.CellarGrant, _ time.Duration) (string, error) {
		signed++
		return g.DB + "/" + g.PermSHA256, nil
	}
	g := auth.CellarGrant{DB: "db-1", WS: "ws-1", CellarID: "local", PermSHA256: "p1"}
	for range 3 {
		if _, err := tk.Token(g); err != nil {
			t.Fatal(err)
		}
	}
	if signed != 1 {
		t.Fatalf("signed %d times for one grant, want 1", signed)
	}
	g.PermSHA256 = "p2"
	if tok, _ := tk.Token(g); tok != "db-1/p2" || signed != 2 {
		t.Fatalf("changed permissions reused a token: %q after %d signs", tok, signed)
	}
}

func TestTokens_RenewsBeforeExpiryUnderSteadyUse(t *testing.T) {
	clock := time.Unix(0, 0)
	signed := 0
	tk := NewTokens()
	tk.now = func() time.Time { return clock }
	tk.sign = func(auth.CellarGrant, time.Duration) (string, error) {
		signed++
		return fmt.Sprint("tok-", signed), nil
	}
	g := auth.CellarGrant{DB: "db-1"}

	// A request every 10s never lets the cache go idle.
	var last string
	for range 12 {
		tok, err := tk.Token(g)
		if err != nil {
			t.Fatal(err)
		}
		last = tok
		clock = clock.Add(10 * time.Second)
	}
	if signed < 2 || last == "tok-1" {
		t.Fatalf("after 120s of steady use the first token is still served (%d signs)", signed)
	}
}

// signWith signs g with priv, standing in for the backend's KMS signer.
func signWith(t *testing.T, priv *rsa.PrivateKey, g auth.CellarGrant) string {
	t.Helper()
	c := auth.CellarClaims{CellarGrant: g, RegisteredClaims: jwt.RegisteredClaims{
		Issuer:    auth.Issuer,
		Audience:  jwt.ClaimStrings{auth.CellarAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestAuthenticate(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	perms := `[{"Action":"select","Effect":"allow"}]`
	good := auth.CellarGrant{DB: "db-1", WS: "ws-1", CellarID: "local", PermSHA256: PermHash([]byte(perms))}
	wrongDB, wrongCel := good, good
	wrongDB.DB, wrongCel.CellarID = "db-2", "cellar-2"

	var seen auth.CellarGrant
	mux := http.NewServeMux()
	mux.Handle("POST /dbs/{id}/execute", Authenticate(&priv.PublicKey, "local")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = GrantFrom(r.Context())
	})))

	cases := []struct {
		name   string
		token  string
		perms  string
		wantOK bool
	}{
		{"valid", signWith(t, priv, good), perms, true},
		{"no token", "", perms, false},
		{"other db", signWith(t, priv, wrongDB), perms, false},
		{"other cellar", signWith(t, priv, wrongCel), perms, false},
		{"widened permissions", signWith(t, priv, good), `[{"Action":"manage","Effect":"allow"}]`, false},
	}
	for _, c := range cases {
		seen = auth.CellarGrant{}
		r := httptest.NewRequest("POST", "/dbs/db-1/execute", nil)
		r.Header.Set("Authorization", "Bearer "+c.token)
		r.Header.Set(PermHeader, c.perms)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if ok := w.Code == http.StatusOK; ok != c.wantOK {
			t.Errorf("%s: status %d, want ok=%v", c.name, w.Code, c.wantOK)
		}
		if c.wantOK && seen != good {
			t.Errorf("%s: handler saw grant %+v, want %+v", c.name, seen, good)
		}
	}
}
