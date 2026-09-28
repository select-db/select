package cellar

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newFile creates a managed database in a temp dir and returns its path and id.
func newFile(t *testing.T) (string, string) {
	t.Helper()
	id := uuid.NewString()
	path := filepath.Join(t.TempDir(), id+".db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE blob (b BLOB)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return path, id
}

func TestOpenRefuses(t *testing.T) {
	path, id := newFile(t)

	_, err := Open(path, Grant{DatasourceID: id})
	require.Error(t, err, "a grant without a size cap")

	missingPath := filepath.Join(t.TempDir(), uuid.NewString()+".db")
	conn, err := Open(missingPath, Grant{DatasourceID: id, MaxBytes: 1 << 20})
	require.NoError(t, err)
	require.Error(t, conn.DB.Ping(), "a database that does not exist")
	require.NoFileExists(t, missingPath)
}
