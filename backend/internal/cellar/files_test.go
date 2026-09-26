package cellar

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine"
	"github.com/stretchr/testify/require"
)

func newFile(t *testing.T) (*Files, string) {
	t.Helper()
	files, id := &Files{Dir: t.TempDir()}, uuid.NewString()
	db, err := sql.Open("sqlite", filepath.Join(files.Dir, id+".db"))
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE blob (b BLOB)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return files, id
}

// statementConn is a connection set up the way the engine sets one up for a
// user statement.
func statementConn(t *testing.T, conn engine.Conn) *sql.Conn {
	t.Helper()
	c, err := conn.DB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	require.NoError(t, conn.Prepare(c, "INSERT INTO blob VALUES (NULL)"))
	return c
}

func TestOpenCapsTheSize(t *testing.T) {
	files, id := newFile(t)
	conn, err := files.Open(Grant{DatasourceID: id, MaxBytes: 256 << 10})
	require.NoError(t, err)

	c := statementConn(t, conn)
	ctx := context.Background()
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(64 * 1024))")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(1024 * 1024))")
	require.ErrorContains(t, err, "full")
}

func TestOpenRefusesForbiddenStatements(t *testing.T) {
	files, id := newFile(t)
	conn, err := files.Open(Grant{DatasourceID: id, MaxBytes: 1 << 20})
	require.NoError(t, err)
	c, err := conn.DB.Conn(context.Background())
	require.NoError(t, err)
	defer c.Close()
	require.ErrorIs(t, conn.Prepare(c, "PRAGMA temp_store_directory = '/tmp'"), ErrForbiddenStatement)
}

func TestOpenRefuses(t *testing.T) {
	files, id := newFile(t)

	_, err := files.Open(Grant{DatasourceID: id})
	require.Error(t, err, "a grant without a size cap")

	_, err = files.Open(Grant{DatasourceID: "../" + id, MaxBytes: 1 << 20})
	require.Error(t, err, "an id that is not a uuid")

	missing := uuid.NewString()
	conn, err := files.Open(Grant{DatasourceID: missing, MaxBytes: 1 << 20})
	require.NoError(t, err)
	require.Error(t, conn.DB.Ping(), "a database that does not exist")
	require.NoFileExists(t, filepath.Join(files.Dir, missing+".db"))
}
