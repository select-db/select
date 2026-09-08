package graph

import (
	"slices"
	"sort"
	"testing"
)

// sharedGraph lays out a workspace with a shared database at the root, a shared
// one two folders down, and a local one beside it:
//
//	root/
//	  shared-root      (proxified)
//	  local            (not proxified)
//	  team/
//	    reports/
//	      shared-deep  (proxified)
func sharedGraph() *Graph {
	g := indexedGraph()

	team := &FolderNode{ID: "team", URI: "team", FolderID: "root"}
	reports := &FolderNode{ID: "reports", URI: "reports", FolderID: "team"}
	g.attach(team)
	g.attach(reports)

	g.attach(&DBInstanceNode{
		ID: "shared-root", URI: "uri/shared-root", Name: "Shared root",
		Proxified: true, FolderID: "root", WorkspaceID: "ws-1",
	})
	g.attach(&DBInstanceNode{
		ID: "local", URI: "uri/local", Name: "Local",
		FolderID: "root", WorkspaceID: "ws-1",
	})
	g.attach(&DBInstanceNode{
		ID: "shared-deep", URI: "uri/shared-deep", Name: "Shared deep",
		Proxified: true, FolderID: "reports", WorkspaceID: "ws-1",
	})

	return g
}

func sharedIDs(refs []DatabaseRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	sort.Strings(ids)
	return ids
}

func assertShared(t *testing.T, got []DatabaseRef, want ...string) {
	t.Helper()

	gotIDs := sharedIDs(got)
	sort.Strings(want)
	if !slices.Equal(gotIDs, want) {
		t.Fatalf("got %v, want %v", gotIDs, want)
	}
}

func TestSharedDatabasesUnder_FindsWhatADeleteWouldRevoke(t *testing.T) {
	g := sharedGraph()

	// A database names itself, by config id or by URI: the tree hands the
	// frontend one and the config file the other.
	assertShared(t, g.SharedDatabasesUnder([]string{"shared-root"}), "shared-root")
	assertShared(t, g.SharedDatabasesUnder([]string{"uri/shared-root"}), "shared-root")

	// Deleting a folder deletes everything below it, however deep.
	assertShared(t, g.SharedDatabasesUnder([]string{"team"}), "shared-deep")

	// A database whose credentials live in the workspace has nothing to revoke.
	assertShared(t, g.SharedDatabasesUnder([]string{"local"}))

	// No ids asks about the whole workspace, which is the connections screen's
	// question: what does anything here still point at?
	assertShared(t, g.SharedDatabasesUnder(nil), "shared-root", "shared-deep")
}

func TestSharedDatabasesUnder_ReturnsEachDatabaseOnce(t *testing.T) {
	g := sharedGraph()

	// A selection can name a folder and something inside it, and a database
	// hangs from both its folder and the workspace's flat list. Neither is a
	// reason to revoke twice.
	got := g.SharedDatabasesUnder([]string{"root", "team", "reports", "shared-deep", "uri/shared-deep"})
	assertShared(t, got, "shared-root", "shared-deep")
}

func TestSharedDatabasesUnder_SkipsIdsTheGraphDoesNotKnow(t *testing.T) {
	g := sharedGraph()

	// A stale selection must not quietly answer "nothing to revoke" for the
	// ids that are still real.
	assertShared(t, g.SharedDatabasesUnder([]string{"gone", "shared-root"}), "shared-root")
	assertShared(t, g.SharedDatabasesUnder([]string{"gone"}))
}

func TestSharedDatabasesUnder_NamesEachDatabase(t *testing.T) {
	g := sharedGraph()

	got := g.SharedDatabasesUnder([]string{"shared-root"})
	if len(got) != 1 || got[0].Name != "Shared root" {
		t.Fatalf("expected the database's name for the confirmation, got %+v", got)
	}
}

func TestSharedDatabasesUnder_DescendsIntoADatabasesOwnFolders(t *testing.T) {
	g := sharedGraph()

	db, _ := g.lookup("shared-root").(*DBInstanceNode)
	schema := &DBInstanceItemNode{ID: "public", ParentID: "shared-root"}
	schema.AddChild(&DBInstanceItemNode{ID: "public.orders", ParentID: "public"})
	db.AddChild(schema)

	// A database directory can hold folders of its own, and one of those can
	// hold another database.
	inside := &FolderNode{ID: "inside", URI: "inside", FolderID: "shared-root"}
	db.AddChild(inside)
	nested := &DBInstanceNode{
		ID: "shared-nested", URI: "uri/shared-nested", Name: "Shared nested",
		Proxified: true, FolderID: "inside", WorkspaceID: "ws-1",
	}
	inside.AddChild(nested)

	assertShared(t, g.SharedDatabasesUnder([]string{"shared-root"}), "shared-root", "shared-nested")
}
