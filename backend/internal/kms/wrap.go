// Package kms does envelope encryption: a fresh DEK per Wrap encrypts the
// plaintext (XChaCha20-Poly1305), and the DEK is wrapped under a KEK held by a
// KEKProvider.
//
// Blob layout (self-describing, store as-is in a bytea column):
//
//	version(1) | kekIDLen(1) | kekID | wrappedDEKLen(2 BE) | wrappedDEK | nonce(24) | ciphertext
package kms

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	blobVersion = 0x01
	dekSize     = chacha20poly1305.KeySize    // 32
	nonceSize   = chacha20poly1305.NonceSizeX // 24
)

var (
	ErrInvalidBlob = errors.New("kms: malformed blob")
	ErrKeyMismatch = errors.New("kms: blob wrapped under a different KEK")
)

// Wrapper turns plaintext into self-describing ciphertext blobs and back.
type Wrapper struct {
	provider KEKProvider
}

func NewWrapper(provider KEKProvider) *Wrapper { return &Wrapper{provider: provider} }

// Wrap returns a storable blob. aad (e.g. workspaceID||datasourceID) is bound to
// the ciphertext and must be passed identically to Unwrap.
func (w *Wrapper) Wrap(ctx context.Context, plaintext, aad []byte) ([]byte, error) {
	dek, wrappedDEK, err := w.provider.NewDEK(ctx)
	if err != nil {
		return nil, fmt.Errorf("kms: new dek: %w", err)
	}
	defer zero(dek)
	if len(dek) != dekSize {
		return nil, fmt.Errorf("kms: dek must be %d bytes, got %d", dekSize, len(dek))
	}

	aead, err := chacha20poly1305.NewX(dek)
	if err != nil {
		return nil, fmt.Errorf("kms: aead: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("kms: gen nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)

	return encodeBlob(w.provider.KeyID(), wrappedDEK, nonce, ciphertext)
}

// Unwrap reverses Wrap. aad must match or decryption fails.
func (w *Wrapper) Unwrap(ctx context.Context, blob, aad []byte) ([]byte, error) {
	kekID, wrappedDEK, nonce, ciphertext, err := decodeBlob(blob)
	if err != nil {
		return nil, err
	}
	if kekID != w.provider.KeyID() {
		return nil, fmt.Errorf("%w: blob=%q active=%q", ErrKeyMismatch, kekID, w.provider.KeyID())
	}

	dek, err := w.provider.UnwrapDEK(ctx, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("kms: unwrap dek: %w", err)
	}
	defer zero(dek)

	aead, err := chacha20poly1305.NewX(dek)
	if err != nil {
		return nil, fmt.Errorf("kms: aead: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("kms: open: %w", err)
	}
	return plaintext, nil
}

func encodeBlob(kekID string, wrappedDEK, nonce, ciphertext []byte) ([]byte, error) {
	if len(kekID) > 255 {
		return nil, fmt.Errorf("kms: kek id > 255 bytes")
	}
	if len(wrappedDEK) > 0xFFFF {
		return nil, fmt.Errorf("kms: wrapped dek > 65535 bytes")
	}
	out := make([]byte, 0, 1+1+len(kekID)+2+len(wrappedDEK)+len(nonce)+len(ciphertext))
	out = append(out, blobVersion, byte(len(kekID)))
	out = append(out, kekID...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(wrappedDEK)))
	out = append(out, wrappedDEK...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decodeBlob(blob []byte) (kekID string, wrappedDEK, nonce, ciphertext []byte, err error) {
	r := blob
	if len(r) < 2 || r[0] != blobVersion {
		return "", nil, nil, nil, ErrInvalidBlob
	}
	idLen := int(r[1])
	r = r[2:]
	if len(r) < idLen+2 {
		return "", nil, nil, nil, ErrInvalidBlob
	}
	kekID = string(r[:idLen])
	r = r[idLen:]

	wLen := int(binary.BigEndian.Uint16(r))
	r = r[2:]
	if len(r) < wLen+nonceSize {
		return "", nil, nil, nil, ErrInvalidBlob
	}
	wrappedDEK = r[:wLen]
	r = r[wLen:]
	nonce = r[:nonceSize]
	ciphertext = r[nonceSize:]
	return kekID, wrappedDEK, nonce, ciphertext, nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
