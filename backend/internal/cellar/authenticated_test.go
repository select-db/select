package cellar

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

// signWith signs a cellar token with priv, standing in for the backend's KMS signer.
func signWith(t *testing.T, priv *rsa.PrivateKey) string {
	t.Helper()
	c := jwt.RegisteredClaims{
		Issuer:    auth.Issuer,
		Audience:  jwt.ClaimStrings{Audience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestAuthenticated(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	good := Grant{WorkspaceID: "ws-1", CellarID: "local", MaxBytes: 1 << 20}
	goodHeader, _ := good.Encode()
	elsewhere, _ := Grant{WorkspaceID: "ws-1", CellarID: "cellar-2"}.Encode()

	var seen Grant
	mux := http.NewServeMux()
	mux.Handle("POST /dbs/{id}/execute", Authenticated(&priv.PublicKey, "local")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = GetGrant(r)
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
		{"malformed grant", signWith(t, priv), "not base64!", false},
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
		want.DatasourceID = "db-1"
		if c.wantOK && !reflect.DeepEqual(seen, want) {
			t.Errorf("%s: handler saw grant %+v, want %+v", c.name, seen, want)
		}
	}
}
