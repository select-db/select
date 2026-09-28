package datasource_test

import (
	"net/http"
	"testing"
	"time"

	"backend/e2e"
	"backend/internal/datasource/cellarclient"

	"github.com/stretchr/testify/require"
)

func TestForkManaged(t *testing.T) {
	fixture := newManagedFixture(t)
	sourceID := createNotes(t, fixture)

	forkID := createAsOwner(t, fixture, "/datasources/"+sourceID+"/fork", nil)
	rows, failure := e2e.Execute(t, fixture, forkID, "SELECT body FROM note")
	require.Nil(t, failure)
	require.Equal(t, [][]any{{"kept"}}, rows, "a fork holds the source's data")
	var name string
	require.NoError(t, fixture.Conn.QueryRow(`SELECT name FROM app.datasource WHERE id = $1`, forkID).Scan(&name))
	require.Equal(t, "notes (fork)", name)
}

func TestForkAtAPointInTime(t *testing.T) {
	fixture := newManagedFixture(t)
	sourceID := createNotes(t, fixture)
	forkPath := "/datasources/" + sourceID + "/fork"

	anHourAgo := time.Now().Add(-time.Hour).Format(time.RFC3339)
	status, responseBody := callAsOwner(t, fixture, http.MethodPost, forkPath, map[string]any{"at": anHourAgo})
	require.Equal(t, http.StatusBadRequest, status, "the source did not exist an hour ago: %s", responseBody)

	aMonthAgo := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	status, _ = callAsOwner(t, fixture, http.MethodPost, forkPath, map[string]any{"at": aMonthAgo})
	require.Equal(t, http.StatusBadRequest, status, "outside the plan's window")
}

func TestForkManagedWithoutCellar(t *testing.T) {
	fixture := newManagedFixture(t)
	sourceID := createNotes(t, fixture)
	cellarclient.URL = ""

	requireDisabled(t, fixture, http.MethodPost, "/datasources/"+sourceID+"/fork")
}
