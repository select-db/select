package cellar

import (
	"database/sql"
	"testing"

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
		require.NoErrorf(t, CheckStatement(sql), "%s", sql)
	}
	for _, sql := range []string{
		"PRAGMA temp_store_directory = '/tmp'",
		"PRAGMA [writable_schema] = 1",
		"PRAGMA `cache_size` = 1",
		"SELECT 1; PRAGMA main.max_page_count = 1",
		"PRAGMA",
		"SELECT * FROM pragma_function_list",
	} {
		require.ErrorIsf(t, CheckStatement(sql), ErrForbiddenStatement, "%s", sql)
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
		require.ErrorIsf(t, CheckStatement("PRAGMA "+name), ErrForbiddenStatement, "PRAGMA %s", name)
		require.ErrorIsf(t, CheckStatement("SELECT * FROM pragma_"+name), ErrForbiddenStatement, "pragma_%s", name)
	}
	require.NoError(t, rows.Err())
	require.Greater(t, n, 50)
}
