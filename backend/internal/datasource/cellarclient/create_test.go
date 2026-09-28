package cellarclient_test

import (
	"context"
	"testing"

	"backend/e2e"
	"backend/internal/cellar"
	"backend/internal/datasource/cellarclient"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

// newDSN is the DSN of a fresh database id on the test cellar, as the backend builds it.
func newDSN(t *testing.T) string {
	t.Helper()
	fixture := e2e.Setup(t)
	e2e.ServeCellar(t)
	return cellarclient.DSN(cellarclient.CellarID, uuid.NewString(), fixture.Actor.WorkspaceID, 1<<20, 0)
}

func TestCreate(t *testing.T) {
	dsn := newDSN(t)

	size, err := cellarclient.Create(context.Background(), dsn, "", "")
	require.NoError(t, err)
	require.Positive(t, size)

	_, err = cellarclient.Create(context.Background(), dsn, "", "")
	var coded *arrowstream.Error
	require.ErrorAs(t, err, &coded, "the cellar's refusal reaches the caller with its code")
	require.Contains(t, coded.Message, "already exists")
}

func TestCreateWithoutCellar(t *testing.T) {
	dsn := newDSN(t)
	cellarclient.URL = ""

	_, err := cellarclient.Create(context.Background(), dsn, "", "")
	require.ErrorIs(t, err, cellarclient.ErrOff)
}

func TestCreatePointInTimeIsNotAvailableYet(t *testing.T) {
	dsn := newDSN(t)

	_, err := cellarclient.Create(context.Background(), dsn, uuid.NewString(), "2026-09-28T00:00:00Z")
	var coded *arrowstream.Error
	require.ErrorAs(t, err, &coded)
	require.Equal(t, cellar.CodeDisabled, coded.Code)
}
