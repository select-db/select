package cellar

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateEmpty(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()

	rec := cellar.call("PUT", "/datasources/"+id, CreateRequest{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var stored StoredDatabase
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&stored))
	require.Equal(t, id, stored.ID)
	require.Positive(t, stored.SizeBytes)

	rec = cellar.call("PUT", "/datasources/"+id, CreateRequest{})
	require.Equal(t, http.StatusConflict, rec.Code, "an id that exists is never overwritten")
}

func TestCreateFork(t *testing.T) {
	cellar := newTestCellar(t)
	sourceID, forkID := uuid.NewString(), uuid.NewString()
	cellar.createNotes(t, sourceID)

	rec := cellar.call("PUT", "/datasources/"+forkID, CreateRequest{SourceID: sourceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cellar.exec(t, forkID, "INSERT INTO note VALUES ('c')")
	require.Equal(t, 2, cellar.countNotes(t, sourceID), "a fork never touches its source")
	require.Equal(t, 3, cellar.countNotes(t, forkID))
}

func TestCreateForkOfMissingSource(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()

	rec := cellar.call("PUT", "/datasources/"+id, CreateRequest{SourceID: uuid.NewString()})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NoFileExists(t, filepath.Join(cellar.dir, id+".db"), "a failed fork leaves no file")
}

func TestCreatePointInTimeIsNotAvailableYet(t *testing.T) {
	cellar := newTestCellar(t)
	sourceID := uuid.NewString()
	cellar.createNotes(t, sourceID)

	rec := cellar.call("PUT", "/datasources/"+uuid.NewString(), CreateRequest{SourceID: sourceID, PointInTime: "2026-09-28T00:00:00Z"})
	require.Equal(t, http.StatusNotImplemented, rec.Code)
	require.Equal(t, CodeDisabled, errorCode(t, rec))
}

func TestCreateRefusesAnIDThatIsNotAUUID(t *testing.T) {
	cellar := newTestCellar(t)

	rec := cellar.call("PUT", "/datasources/..%2Fescape", CreateRequest{})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.NotContains(t, string(body), cellar.dir, "an error never names a path")
}
