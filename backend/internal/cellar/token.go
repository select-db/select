package cellar

import (
	"sync"
	"time"

	"backend/internal/auth"
)

// A token lives tokenTTL and is reused for reuseFor: each KMS sign is a remote
// call, and every reused token still has 10s left when it reaches the cellar.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

// Tokens signs the backend's cellar token, reusing it for reuseFor.
type Tokens struct {
	mu         sync.Mutex
	token      string
	reuseUntil time.Time
	sign       func(time.Duration) (string, error)
	now        func() time.Time
}

func NewTokens() *Tokens {
	return &Tokens{sign: auth.SignCellarToken, now: time.Now}
}

// Token returns a signed token. Callers wait on one sign rather than each
// making their own.
func (t *Tokens) Token() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.now().Before(t.reuseUntil) {
		return t.token, nil
	}
	reuseUntil := t.now().Add(reuseFor) // taken before signing, so a slow sign only shortens reuse
	tok, err := t.sign(tokenTTL)
	if err != nil {
		return "", err
	}
	t.token, t.reuseUntil = tok, reuseUntil
	return tok, nil
}
