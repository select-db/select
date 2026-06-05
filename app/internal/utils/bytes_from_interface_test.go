package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBytesFromInterface(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		b, err := BytesFromInterface(nil)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("[]byte returns same", func(t *testing.T) {
		data := []byte("abc")
		b, err := BytesFromInterface(data)
		assert.NoError(t, err)
		assert.Equal(t, data, b)
	})

	t.Run("string converts to []byte", func(t *testing.T) {
		b, err := BytesFromInterface("hello")
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello"), b)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := BytesFromInterface(123)
		assert.ErrorContains(t, err, "unsupported type")
	})
}
