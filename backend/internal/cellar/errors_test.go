package cellar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassify(t *testing.T) {
	path, id := newFile(t)
	grant := Grant{DatasourceID: id, MaxBytes: 256 << 10}
	conn, err := Open(path, grant)
	require.NoError(t, err)
	c := statementConn(t, conn)
	ctx := context.Background()

	_, sqlErr := c.ExecContext(ctx, "SELECT * FROM nope")
	_, fullErr := c.ExecContext(ctx, "INSERT INTO blob VALUES (zeroblob(1024 * 1024))")
	expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer cancel()

	for _, tc := range []struct {
		name     string
		ctx      context.Context
		err      error
		code     string
		contains string
	}{
		{"SQLite's message about the statement", ctx, sqlErr, CodeSQLError, "no such table: nope"},
		{"the size cap", ctx, fullErr, CodeQuotaExceeded, "over its"},
		{"a forbidden statement", ctx, ErrForbiddenStatement, CodeForbiddenStatement, "not allowed"},
		{"the statement timeout", expired, errors.New("interrupted"), CodeTimeout, "60s"},
		{"anything else, by ref only", ctx, errors.New("open " + path + ": denied"), CodeInternal, "ref "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := classify(tc.ctx, tc.err, grant)
			require.Equal(t, tc.code, got.Code)
			require.Contains(t, got.Message, tc.contains)
			require.NotContains(t, got.Message, path, "no message names a path")
		})
	}
}
