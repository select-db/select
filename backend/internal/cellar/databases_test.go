package cellar

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOpenDatabasesReplicatesWhatIsOnDisk(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	require.NoError(t, cellar.databases.Close(context.Background()))

	reopened, err := OpenDatabases(cellar.dir, cellar.replicaDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close(context.Background()) })
	require.NotNil(t, reopened.store.FindDB(cellar.dir+"/"+id+".db"), "a write the last run had not sent reaches the replica")
	require.Contains(t, reopened.lastUsed, id)
}
