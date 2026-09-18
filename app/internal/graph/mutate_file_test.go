package graph

import (
	"context"
	"encoding/json"
	"selectDb/internal/db/generated"
	"testing"
)

func TestMutate_InsertRootFile(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "file",
		"object_id":  "file-1",
		"payload": `{
			"id":"file-1",
			"uri":"file-1",
			"name":"File1",
			"folder_id":"root",
			"workspace_id":"ws-1",
			"edit_mode":"file"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FileNode); !ok {
		t.Errorf("expected FileNode, got %T", nodes[0])
	}
}

func TestMutate_InsertRootFileWithoutFolderId(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "file",
		"object_id":  "file-1",
		"payload": `{
			"id":"file-1",
			"uri":"file-1",
			"folder_id":"",
			"workspace_id":"ws-1",
			"edit_mode":"file"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FileNode); !ok {
		t.Errorf("expected FileNode, got %T", nodes[0])
	}
}

func TestMutate_InsertFolderFile(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	if err := g.Mutate(ctx, generated.MutationCommit{
		ID:        "2",
		Operation: "insert",
		TableName: "folder",
		ObjectID:  "folder-1",
		Payload: `{
			"id":"folder-1",
			"uri":"folder-1",
			"name":"Folder1",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	}); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	raw := map[string]interface{}{
		"ID":         "1",
		"operation":  "insert",
		"table_name": "file",
		"object_id":  "file-1",
		"payload": `{
			"id":"file-1",
			"uri":"file-1",
			"name":"File1",
			"folder_id":"folder-1",
			"workspace_id":"ws-1",
			"edit_mode":"file"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*FileNode); !ok {
		t.Errorf("expected FileNode, got %T", nodes[0])
	}
}

func TestMutate_UpdateFileName(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	if err := g.Mutate(ctx, generated.MutationCommit{
		TableName: "file",
		Operation: "insert",
		ObjectID:  "file-1",
		Payload: `{
			"id":"file-1",
			"uri":"file-1",
			"name":"OldName",
			"folder_id":"root",
			"workspace_id":"ws-1",
			"edit_mode":"file"
		}`,
	}); err != nil {
		t.Fatalf("mutate update failed: %v", err)
	}

	mut := generated.MutationCommit{
		TableName: "file",
		Operation: "update",
		ObjectID:  "file-1",
		Payload: `{
			"id": "file-1",
			"name":"NewName"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Logf("mutate update failed: %v", err)
		t.Fatalf("mutate update failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	file := nodes[0].(*FileNode)
	if file.Name != "NewName" {
		t.Errorf("expected Name=NewName, got %s", file.Name)
	}
}

func TestMutate_DeleteFile(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	g.Mutate(ctx, generated.MutationCommit{
		TableName: "file",
		Operation: "insert",
		ObjectID:  "file-1",
		Payload: `{
			"id":"file-1",
			"uri":"file-1",
			"name":"File1",
			"folder_id":"root",
			"workspace_id":"ws-1",
		}`,
	})

	mut := generated.MutationCommit{
		TableName: "file",
		Operation: "delete",
		ObjectID:  "file-1",
		Payload: `{
			"id":"file-1"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate delete failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	if len(nodes) != 0 {
		t.Errorf("expected node deleted, still found: %+v", nodes)
	}
}

func TestMutate_UpdateFileResult(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	err := g.Mutate(ctx, generated.MutationCommit{
		TableName: "file",
		Operation: "insert",
		ObjectID:  "file-1",
		Payload: `{
            "id":"file-1",
            "uri":"file-1",
            "name":"File1",
            "folder_id":"root",
            "workspace_id":"ws-1",
            "edit_mode":"file"
        }`,
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	mut := generated.MutationCommit{
		TableName: "file",
		Operation: "update",
		ObjectID:  "file-1",
		Payload: `{
			"id": "file-1",
			"queryResults": {
				"db-1": {
					"columns": ["id", "name"],
					"rows": [[1, "foo"], [2, "bar"]],
					"rowCount": 2,
					"durationMs": 123,
					"page": 0,
					"pageSize": 75
				}
			}
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate update failed: %v", err)
	}

	nodes := g.lookupAll([]string{"file-1"})
	file, ok := nodes[0].(*FileNode)
	if !ok {
		t.Fatalf("expected FileNode, got %T", nodes[0])
	}
	if file.QueryResults == nil {
		t.Fatalf("expected QueryResults to be set, got nil")
	}
	res, has := file.QueryResults["db-1"]
	if !has || res == nil {
		t.Fatalf("expected queryResults[\"db-1\"], got %+v", file.QueryResults)
	}
	if res.RowCount != 2 {
		t.Errorf("expected RowCount=2, got %+v", res.RowCount)
	}
	if res.DurationMs != 123 {
		t.Errorf("expected DurationMs=123, got %+v", res.DurationMs)
	}
	if len(res.Columns) != 2 || res.Columns[0] != "id" {
		t.Errorf("unexpected columns: %+v", res.Columns)
	}
	if len(res.Rows) != 2 {
		t.Errorf("unexpected rows: %+v", res.Rows)
	}
}
