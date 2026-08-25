package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

// A node collects its children from a query result, and a query that matched
// nothing leaves a nil slice. The frontend's types promise an array, so what
// goes over the wire has to be one.
func TestEnsureArraysFillsEveryChildSlice(t *testing.T) {
	// A schema whose functions section came back empty — the shape that made
	// the editor throw on `for (const leaf of folder.children)`.
	workspace := &WorkspaceNode{
		DBInstances: []*DBInstanceNode{{
			Name: "local",
			Children: []*DBInstanceItemNode{{
				Type: "schema",
				Name: "public",
				Children: []*DBInstanceItemNode{{
					Type: "functions",
					Name: "Functions",
				}},
			}},
		}},
	}

	ensureArrays(workspace)

	encoded, err := json.Marshal(workspace)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"children":null`) {
		t.Fatalf("a child slice still marshalled as null: %s", encoded)
	}

	functions := workspace.DBInstances[0].Children[0].Children[0]
	if functions.Children == nil {
		t.Error("the empty functions section should carry an empty slice, not nil")
	}
	if functions.Badges == nil {
		t.Error("badges should be an array too")
	}
}

func TestEnsureArraysKeepsExistingChildren(t *testing.T) {
	workspace := &WorkspaceNode{
		Folders: []*FolderNode{{
			Name:  "queries",
			Files: []*FileNode{{Name: "report.sql"}},
		}},
	}

	ensureArrays(workspace)

	if got := len(workspace.Folders[0].Files); got != 1 {
		t.Errorf("expected the existing file to survive, got %d files", got)
	}
}

// Nulls that mean something keep meaning it.
func TestEnsureArraysLeavesGenuineNullsAlone(t *testing.T) {
	workspace := &WorkspaceNode{}

	ensureArrays(workspace)

	encoded, err := json.Marshal(workspace)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"user":null`) {
		t.Errorf("a workspace with no user should still say so: %s", encoded)
	}
}

func TestEnsureArraysHandlesNilWorkspace(t *testing.T) {
	ensureArrays(nil) // Mutate calls this on a graph that may not be built yet.
}
