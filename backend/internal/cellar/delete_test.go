package cellar

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	cellar := newTestCellar(t)
	id, keptID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, id)
	cellar.createNotes(t, keptID)

	rec := cellar.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	entries, err := os.ReadDir(cellar.dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), id), "%s left behind", entry.Name())
	}
	require.Equal(t, 2, cellar.countNotes(t, keptID), "only the named database goes")

	rec = cellar.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "deleting twice is not an error")
}
