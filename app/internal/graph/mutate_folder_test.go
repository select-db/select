package graph

import (
	"context"
	"encoding/json"
	"selectDb/internal/db/generated"
	"testing"
)

func TestMutate_InsertRootFileFolder(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "folder",
		"object_id":  "folder-1",
		"payload": `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"RootFolder",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FolderNode); !ok {
		t.Errorf("expected FolderNode, got %T", nodes[0])
	}
}

func TestMutate_InsertRootDbInstanceFolder(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "folder",
		"object_id":  "folder-1",
		"payload": `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"RootFolder",
			"folder_id":"",
			"workspace_id":"ws-1"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	folder, ok := nodes[0].(*FolderNode)
	if !ok {
		t.Fatalf("expected FolderNode, got %T", nodes[0])
	}
	expectedParentID := "root"
	if folder.FolderID != expectedParentID {
		t.Errorf("expected parent ID %q, got %q", expectedParentID, folder.FolderID)
	}
}

func TestMutate_InsertRootFolderWithoutFolderId(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "folder",
		"object_id":  "folder-1",
		"payload": `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"RootFolder",
			"folder_id":"",
			"workspace_id":"ws-1"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FolderNode); !ok {
		t.Errorf("expected FolderNode, got %T", nodes[0])
	}
}

func TestMutate_InsertNestedFolder(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	g.Mutate(ctx, generated.MutationCommit{
		TableName: "folder",
		Operation: "insert",
		ObjectID:  "folder-1",
		Payload: `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"ParentFolder",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	})

	mut := generated.MutationCommit{
		TableName: "folder",
		Operation: "insert",
		ObjectID:  "folder-2",
		Payload: `{
			"id":"folder-2",
			"uri":"folder-2",
			"name":"ChildFolder",
			"folder_id":"folder-1",
			"workspace_id":"ws-1"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-2"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FolderNode); !ok {
		t.Errorf("expected FolderNode, got %T", nodes[0])
	}
}

func TestMutate_UpdateFolderName(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	g.Mutate(ctx, generated.MutationCommit{
		TableName: "folder",
		Operation: "insert",
		ObjectID:  "folder-1",
		Payload: `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"OldFolder",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	})

	mut := generated.MutationCommit{
		TableName: "folder",
		Operation: "update",
		ObjectID:  "folder-1",
		Payload: `{
			"name":"NewFolder"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate update failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-1"})
	folder := nodes[0].(*FolderNode)
	if folder.Name != "NewFolder" {
		t.Errorf("expected Name=NewFolder, got %s", folder.Name)
	}
}

func TestMutate_DeleteFolder(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	g.Mutate(ctx, generated.MutationCommit{
		TableName: "folder",
		Operation: "insert",
		ObjectID:  "folder-1",
		Payload: `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"Folder1",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	})

	mut := generated.MutationCommit{
		TableName: "folder",
		Operation: "delete",
		ObjectID:  "folder-1",
		Payload: `{
			"id":"folder-1"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate delete failed: %v", err)
	}

	nodes := FindNodesByIds(g.WorkspaceGraph, []string{"folder-1"})
	if len(nodes) != 0 {
		t.Errorf("expected node deleted, still found: %+v", nodes)
	}
}
