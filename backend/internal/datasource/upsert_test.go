package datasource_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertManagedOnlyRenames(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createNotes(t, fixture)

	status, responseBody := callAsOwner(t, fixture, http.MethodPut, "/datasources/"+id,
		map[string]any{"name": "renamed", "db_type": "postgresql", "dsn": "postgres://elsewhere/db"})
	require.Equal(t, http.StatusNoContent, status, string(responseBody))

	var name, dbType string
	require.NoError(t, fixture.Conn.QueryRow(`SELECT name, db_type FROM app.datasource WHERE id = $1`, id).Scan(&name, &dbType))
	require.Equal(t, []string{"renamed", "sqlite"}, []string{name, dbType}, "a managed database only takes a new name")
}
