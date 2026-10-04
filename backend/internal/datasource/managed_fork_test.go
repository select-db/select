package datasource_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"backend/e2e"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
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

func TestForkNeedsManageOnTheSource(t *testing.T) {
	fixture := newManagedFixture(t)
	sourceID := createNotes(t, fixture)
	forkPath := "/datasources/" + sourceID + "/fork"
	body := map[string]any{"workspace_id": fixture.Actor.WorkspaceID}

	rec := e2e.Do(t, fixture.H, http.MethodPost, forkPath, memberToken(t, fixture, sourceID, "select"), body)
	require.Equal(t, http.StatusForbidden, rec.Code, "reading the source is not enough: %s", rec.Body.String())
	e2e.RequireEventStatus(t, fixture.Conn, "datasource", "lifecycle.create", "denied")

	otherID := createNotes(t, fixture)
	rec = e2e.Do(t, fixture.H, http.MethodPost, forkPath, memberToken(t, fixture, otherID, "manage"), body)
	require.Equal(t, http.StatusForbidden, rec.Code, "manage on another database: %s", rec.Body.String())

	rec = e2e.Do(t, fixture.H, http.MethodPost, forkPath, memberToken(t, fixture, sourceID, "manage"), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestForkUnknownSource(t *testing.T) {
	fixture := newManagedFixture(t)

	status, responseBody := callAsOwner(t, fixture, http.MethodPost, "/datasources/"+uuid.NewString()+"/fork", nil)
	require.Equal(t, http.StatusNotFound, status, string(responseBody))
}

func TestForkClassicDatasource(t *testing.T) {
	fixture := newManagedFixture(t)
	status, responseBody := callAsOwner(t, fixture, http.MethodPost, "/datasources", map[string]any{"db_type": "postgresql", "name": "remote", "dsn": e2e.TargetDSN(t, fixture.Conn)})
	require.Equal(t, http.StatusCreated, status, string(responseBody))
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &created))

	status, responseBody = callAsOwner(t, fixture, http.MethodPost, "/datasources/"+created.ID+"/fork", nil)
	require.Equal(t, http.StatusNotFound, status, "only a managed database forks: %s", responseBody)
}

// Parallel creates cannot all take the last slots: the staging stress test
// made twelve databases in a workspace that allows ten.
func TestCreateInParallelStaysUnderTheQuota(t *testing.T) {
	fixture := newManagedFixture(t)
	const attempts, allowed = 16, 10
	statuses := make(chan int, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := callAsOwner(t, fixture, http.MethodPost, "/datasources", map[string]any{"db_type": "sqlite", "name": "parallel"})
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)

	created := 0
	for status := range statuses {
		if status == http.StatusCreated {
			created++
		}
	}
	require.Equal(t, allowed, created, "the solo plan holds ten managed databases")
	var rows int
	require.NoError(t, fixture.Conn.QueryRow(`SELECT count(*) FROM app.datasource WHERE cellar_id IS NOT NULL`).Scan(&rows))
	require.Equal(t, allowed, rows)
}
