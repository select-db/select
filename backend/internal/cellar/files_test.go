package cellar

import (
	"context"
	"database/sql"
	"testing"

	"backend/internal/auth"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine"
	"github.com/stretchr/testify/require"
)

func newFile(t *testing.T) (*Files, string) {
	t.Helper()
	files, id := NewFiles(t.TempDir()), uuid.NewString()
	db, err := sql.Open("sqlite", files.Path(id))
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
	require.NoError(t, conn.Prepare(c))
	return c
}

func TestOpenCapsTheSize(t *testing.T) {
	files, id := newFile(t)
	conn, err := files.Open(auth.CellarGrant{DB: id, MaxBytes: 256 << 10}, nil)
	require.NoError(t, err)

	c := statementConn(t, conn)
	ctx := context.Background()
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(64 * 1024))")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(1024 * 1024))")
	require.ErrorContains(t, err, "full")
}

func TestOpenRefuses(t *testing.T) {
	files, id := newFile(t)

	_, err := files.Open(auth.CellarGrant{DB: id}, nil)
	require.Error(t, err, "a grant without a size cap")

	_, err = files.Open(auth.CellarGrant{DB: "../" + id, MaxBytes: 1 << 20}, nil)
	require.Error(t, err, "an id that is not a uuid")

	missing := uuid.NewString()
	_, err = files.Open(auth.CellarGrant{DB: missing, MaxBytes: 1 << 20}, nil)
	require.Error(t, err, "a database that does not exist")
	require.NoFileExists(t, files.Path(missing))
}
