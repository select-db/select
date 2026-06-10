package datasource

import (
	"context"
	"sync"

	"backend/internal/kms"

	"github.com/google/uuid"
)

var (
	wrapperOnce sync.Once
	wrapperInst *kms.Wrapper
	wrapperErr  error
)

// getWrapper lazily builds and caches the Wrapper from the configured KEK provider.
func getWrapper() (*kms.Wrapper, error) {
	wrapperOnce.Do(func() {
		p, err := kms.NewProvider()
		if err != nil {
			wrapperErr = err
			return
		}
		wrapperInst = kms.New(p)
	})
	return wrapperInst, wrapperErr
}

// fieldAAD binds a ciphertext to its workspace, datasource, and column so a blob
// cannot be swapped between rows, tenants, or fields.
func fieldAAD(workspaceID, id uuid.UUID, field string) []byte {
	return []byte(workspaceID.String() + ":" + id.String() + ":" + field)
}

// encryptField wraps non-empty plaintext; empty stays NULL (nil bytea).
func encryptField(ctx context.Context, w *kms.Wrapper, plaintext string, aad []byte) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	return w.Wrap(ctx, []byte(plaintext), aad)
}

// decryptField unwraps a blob; empty/NULL stays "".
func decryptField(ctx context.Context, w *kms.Wrapper, blob, aad []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	pt, err := w.Unwrap(ctx, blob, aad)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
