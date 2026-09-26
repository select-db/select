package cellar

import (
	"context"
	"database/sql"
	"testing"

	"github.com/selectDb/dialect/engine"
	"github.com/stretchr/testify/require"
)

func TestCheckStatement(t *testing.T) {
	for _, sql := range []string{
		"SELECT 'PRAGMA temp_store_directory' AS s",
		"-- PRAGMA writable_schema = 1\nSELECT 1",
		"PRAGMA table_info(note)",
		"pragma MAIN.Index_List('note')",
		"SELECT * FROM pragma_table_xinfo('note')",
	} {
		require.NoErrorf(t, checkStatement(sql), "%s", sql)
	}
	for _, sql := range []string{
		"PRAGMA temp_store_directory = '/tmp'",
		"PRAGMA [writable_schema] = 1",
		"PRAGMA `cache_size` = 1",
		"SELECT 1; PRAGMA main.max_page_count = 1",
		"PRAGMA",
		"SELECT * FROM pragma_function_list",
	} {
		require.ErrorIsf(t, checkStatement(sql), ErrForbiddenStatement, "%s", sql)
	}
}

func TestCheckStatementRefusesEveryOtherPragma(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	rows, err := db.Query("SELECT name FROM pragma_pragma_list")
	require.NoError(t, err)
	defer rows.Close()
	var n int
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		n++
		if readOnlyPragmas[name] {
			continue
		}
		require.ErrorIsf(t, checkStatement("PRAGMA "+name), ErrForbiddenStatement, "PRAGMA %s", name)
		require.ErrorIsf(t, checkStatement("SELECT * FROM pragma_"+name), ErrForbiddenStatement, "pragma_%s", name)
	}
	require.NoError(t, rows.Err())
	require.Greater(t, n, 50)
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

func TestIsolationCapsTheSize(t *testing.T) {
	dir, id := newFile(t)
	conn, err := Open(dir, Grant{DatasourceID: id, MaxBytes: 256 << 10})
	require.NoError(t, err)

	c := statementConn(t, conn)
	ctx := context.Background()
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(64 * 1024))")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(1024 * 1024))")
	require.ErrorContains(t, err, "full")
}

func TestIsolationRefusesForbiddenStatements(t *testing.T) {
	dir, id := newFile(t)
	conn, err := Open(dir, Grant{DatasourceID: id, MaxBytes: 1 << 20})
	require.NoError(t, err)
	c, err := conn.DB.Conn(context.Background())
	require.NoError(t, err)
	defer c.Close()
	require.ErrorIs(t, conn.Prepare(c, "PRAGMA temp_store_directory = '/tmp'"), ErrForbiddenStatement)
}
