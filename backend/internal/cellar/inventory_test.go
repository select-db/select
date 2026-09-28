package cellar

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInventory(t *testing.T) {
	cellar := newTestCellar(t)
	firstID, secondID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, firstID)
	cellar.createNotes(t, secondID)
	require.NoError(t, os.WriteFile(filepath.Join(cellar.dir, ".tmp-"+uuid.NewString()+".db"), nil, 0o600))

	rec := cellar.call("GET", "/datasources", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var inventory []StoredDatabase
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&inventory))
	listedIDs := []string{}
	for _, stored := range inventory {
		listedIDs = append(listedIDs, stored.ID)
		require.Positive(t, stored.SizeBytes)
	}
	require.ElementsMatch(t, []string{firstID, secondID}, listedIDs, "WAL files and temporary copies are not databases")
}
