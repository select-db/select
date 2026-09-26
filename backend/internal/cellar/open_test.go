package cellar

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newFile creates a managed database in a temp dir and returns the dir and its id.
func newFile(t *testing.T) (string, string) {
	t.Helper()
	dir, id := t.TempDir(), uuid.NewString()
	db, err := sql.Open("sqlite", filepath.Join(dir, id+".db"))
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE blob (b BLOB)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return dir, id
}

func TestOpenRefuses(t *testing.T) {
	dir, id := newFile(t)

	_, err := Open(dir, Grant{DatasourceID: id})
	require.Error(t, err, "a grant without a size cap")

	_, err = Open(dir, Grant{DatasourceID: "../" + id, MaxBytes: 1 << 20})
	require.Error(t, err, "an id that is not a uuid")

	missing := uuid.NewString()
	conn, err := Open(dir, Grant{DatasourceID: missing, MaxBytes: 1 << 20})
	require.NoError(t, err)
	require.Error(t, conn.DB.Ping(), "a database that does not exist")
	require.NoFileExists(t, filepath.Join(dir, missing+".db"))
}
