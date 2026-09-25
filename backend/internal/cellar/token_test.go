package cellar

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/selectDb/dialect/core"
)

func TestTokens_RenewsBeforeExpiryUnderSteadyUse(t *testing.T) {
	clock := time.Unix(0, 0)
	signed := 0
	tk := NewTokens()
	tk.now = func() time.Time { return clock }
	tk.sign = func(time.Duration) (string, error) {
		signed++
		return fmt.Sprint("tok-", signed), nil
	}

	// A request every 10s for 120s: a sign every reuseFor, never one per request.
	var last string
	for range 12 {
		tok, err := tk.Token()
		if err != nil {
			t.Fatal(err)
		}
		last = tok
		clock = clock.Add(10 * time.Second)
	}
	if signed != 3 || last != "tok-3" {
		t.Fatalf("got %d signs, last %q; want 3 signs, last tok-3", signed, last)
	}
}

// signWith signs a cellar token with priv, standing in for the backend's KMS signer.
func signWith(t *testing.T, priv *rsa.PrivateKey) string {
	t.Helper()
	c := jwt.RegisteredClaims{
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

func TestAuthenticate(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	good := Grant{WS: "ws-1", CellarID: "local", MaxBytes: 1 << 20, Perms: []core.PermissionEntry{{Action: "select", Effect: "allow"}}}
	goodHeader, _ := encodeGrant(good)
	elsewhere, _ := encodeGrant(Grant{WS: "ws-1", CellarID: "cellar-2"})

	var seen Grant
	mux := http.NewServeMux()
	mux.Handle("POST /dbs/{id}/execute", Authenticate(&priv.PublicKey, "local")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = GrantFrom(r.Context())
	})))

	cases := []struct {
		name, token, grant string
		wantOK             bool
	}{
		{"valid", signWith(t, priv), goodHeader, true},
		{"no token", "", goodHeader, false},
		{"other key", signWith(t, other), goodHeader, false},
		{"other cellar", signWith(t, priv), elsewhere, false},
		{"no grant", signWith(t, priv), "", false},
	}
	for _, c := range cases {
		seen = Grant{}
		r := httptest.NewRequest("POST", "/dbs/db-1/execute", nil)
		r.Header.Set("Authorization", "Bearer "+c.token)
		r.Header.Set(GrantHeader, c.grant)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if ok := w.Code == http.StatusOK; ok != c.wantOK {
			t.Errorf("%s: status %d, want ok=%v", c.name, w.Code, c.wantOK)
		}
		want := good
		want.DB = "db-1"
		if c.wantOK && !reflect.DeepEqual(seen, want) {
			t.Errorf("%s: handler saw grant %+v, want %+v", c.name, seen, want)
		}
	}
}
