package cellarclient_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"backend/internal/datasource/cellarclient"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	dsn := newDSN(t)
	_, err := cellarclient.Create(context.Background(), dsn, "", "")
	require.NoError(t, err)

	databaseFile, err := cellarclient.Download(context.Background(), dsn)
	require.NoError(t, err)
	defer databaseFile.Close()
	content, err := io.ReadAll(databaseFile)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(content), "SQLite format 3"))
}

func TestDownloadMissing(t *testing.T) {
	dsn := newDSN(t)

	_, err := cellarclient.Download(context.Background(), dsn)
	var coded *arrowstream.Error
	require.ErrorAs(t, err, &coded)
	require.Contains(t, coded.Message, "not found")
}
