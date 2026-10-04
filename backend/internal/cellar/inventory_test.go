package cellar

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func inventoryOf(t *testing.T, cellar testCellar) map[string]StoredDatabase {
	t.Helper()
	rec := cellar.call("GET", "/datasources", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var inventory []StoredDatabase
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&inventory))
	byID := map[string]StoredDatabase{}
	for _, stored := range inventory {
		byID[stored.ID] = stored
	}
	return byID
}

// evictedNotes is a database that only the bucket holds.
func evictedNotes(t *testing.T, cellar testCellar) string {
	t.Helper()
	id := uuid.NewString()
	cellar.createNotes(t, id)
	databases.onDisk[id].lastUsed = time.Now().Add(-time.Hour)
	databases.rest(context.Background(), time.Now().Add(restAfter))
	databases.evict(shortFor(1))
	require.NotContains(t, databases.onDisk, id)
	return id
}

func TestInventoryListsColdDatabases(t *testing.T) {
	cellar := newTestCellar(t)
	coldID, warmID := evictedNotes(t, cellar), uuid.NewString()
	cellar.createNotes(t, warmID)

	inventory := inventoryOf(t, cellar)

	require.Len(t, inventory, 2)
	require.True(t, inventory[coldID].Cold, "the bucket alone holds it")
	require.False(t, inventory[coldID].ModifiedAt.IsZero(), "dated by its newest object")
	require.WithinDuration(t, time.Now(), inventory[coldID].ModifiedAt, time.Minute)
	require.False(t, inventory[warmID].Cold)
	require.Positive(t, inventory[warmID].SizeBytes)
	require.False(t, inventory[warmID].ModifiedAt.IsZero())
}

func TestInventoryIgnoresWhatIsNotADatabaseInTheBucket(t *testing.T) {
	cellar := newTestCellar(t)
	require.NoError(t, os.MkdirAll(filepath.Join(cellar.bucketDir, "dbs", "not-a-uuid"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cellar.bucketDir, "dbs", "stray-file"), nil, 0o600))

	require.Empty(t, inventoryOf(t, cellar))
}

func TestInventoryFailsWhenTheBucketCannotBeListed(t *testing.T) {
	cellar := newTestCellar(t)
	cellar.createNotes(t, uuid.NewString())
	// A file where the folder of databases should be.
	require.NoError(t, os.WriteFile(filepath.Join(cellar.bucketDir, "dbs"), nil, 0o600))

	rec := cellar.call("GET", "/datasources", nil)

	require.Equal(t, http.StatusInternalServerError, rec.Code, "a short list would have the reconciler purge what it does not name")
}

func TestDeleteRemovesAColdDatabaseFromTheBucket(t *testing.T) {
	cellar := newTestCellar(t)
	id := evictedNotes(t, cellar)
	require.Contains(t, inventoryOf(t, cellar), id)

	rec := cellar.call("DELETE", "/datasources/"+id, nil)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.NotContains(t, inventoryOf(t, cellar), id)
	require.NoDirExists(t, filepath.Join(cellar.bucketDir, "dbs", id))
}
