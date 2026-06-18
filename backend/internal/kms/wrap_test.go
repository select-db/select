package kms

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

// newTestWrapper builds a Wrapper over a localProvider with a fresh random KEK.
func newTestWrapper(t *testing.T) *Wrapper {
	t.Helper()
	kek := make([]byte, dekSize)
	if _, err := rand.Read(kek); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KEKEnv, base64.StdEncoding.EncodeToString(kek))
	p, err := newLocalProvider(KEKEnv)
	if err != nil {
		t.Fatal(err)
	}
	return NewWrapper(p)
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	plaintext := []byte("postgres://user:pass@host:5432/db?sslmode=require")
	aad := []byte("workspace-123:datasource-456")

	blob, err := w.Wrap(ctx, plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Fatal("plaintext leaked into blob")
	}

	got, err := w.Unwrap(ctx, blob, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip mismatch: got %q", got)
	}
}

func TestUnwrapWrongAADFails(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()

	blob, err := w.Wrap(ctx, []byte("secret"), []byte("ws-1:ds-1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Unwrap(ctx, blob, []byte("ws-1:ds-2")); err == nil {
		t.Fatal("expected failure on aad mismatch")
	}
}

func TestUnwrapTamperedCiphertextFails(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	aad := []byte("ws:ds")

	blob, err := w.Wrap(ctx, []byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}
	blob[len(blob)-1] ^= 0xFF // flip a ciphertext byte
	if _, err := w.Unwrap(ctx, blob, aad); err == nil {
		t.Fatal("expected failure on tampered ciphertext")
	}
}

func TestUnwrapWrongKeyFails(t *testing.T) {
	a := newTestWrapper(t)
	b := newTestWrapper(t) // different random KEK -> different KeyID
	ctx := context.Background()
	aad := []byte("ws:ds")

	blob, err := a.Wrap(ctx, []byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Unwrap(ctx, blob, aad); err == nil {
		t.Fatal("expected failure unwrapping under a different KEK")
	}
}

func TestUnwrapMalformedBlob(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	cases := [][]byte{
		nil,
		{},
		{0x02, 0x00},             // wrong version
		{blobVersion},            // truncated
		{blobVersion, 0xFF, 'x'}, // idLen claims 255, body too short
	}
	for i, c := range cases {
		if _, err := w.Unwrap(ctx, c, nil); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}
