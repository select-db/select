package graph

import (
	"context"
	"encoding/json"
	"selectDb/internal/db/generated"
	"testing"
)

func TestMutate_InsertDbInstance(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	mut := generated.MutationCommit{
		TableName: "db_instance",
		Operation: "insert",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1",
			"uri":"db-1",
			"name":"DB1",
			"db_type":"sqlite",
			"folder_id":"root",
			"workspace_id":"ws-1",
			"dsn":"file:test.db"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*DBInstanceNode); !ok {
		t.Errorf("expected DBInstanceNode, got %T", nodes[0])
	}
}

func TestMutate_InsertDbInstanceWithoutDSN(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	mut := generated.MutationCommit{
		TableName: "db_instance",
		Operation: "insert",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1",
			"uri":"db-1",
			"name":"DB1",
			"db_type":"sqlite",
			"folder_id":"root",
			"workspace_id":"ws-1"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*DBInstanceNode); !ok {
		t.Errorf("expected DBInstanceNode, got %T", nodes[0])
	}
}

func TestMutate_InsertDbInstanceWithoutFolderId(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	mut := generated.MutationCommit{
		TableName: "db_instance",
		Operation: "insert",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1",
			"uri":"db-1",
			"name":"DB1",
			"db_type":"sqlite",
			"folder_id":"",
			"workspace_id":"ws-1",
			"dsn":"file:test.db"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0].(*DBInstanceNode); !ok {
		t.Errorf("expected DBInstanceNode, got %T", nodes[0])
	}
}

func TestMutate_InsertFolderDbInstance(t *testing.T) {
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
		"table_name": "db_instance",
		"object_id":  "db-1",
		"payload": `{
			"id":"db-1",
			"uri":"db-1",
			"name":"DB1",
			"db_type":"sqlite",
			"folder_id":"folder-1",
			"workspace_id":"ws-1",
			"dsn":"file:test.db"
		}`,
	}
	data, _ := json.Marshal(raw)
	var mut generated.MutationCommit
	json.Unmarshal(data, &mut)

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	db, ok := nodes[0].(*DBInstanceNode)
	if !ok {
		t.Errorf("expected DBInstanceNode, got %T", nodes[0])
	}
	nodes = g.lookupAll([]string{db.FolderID})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 parent folder node, got %d", len(nodes))
	}
	parentFolder, ok := nodes[0].(*FolderNode)
	if !ok {
		t.Errorf("expected DBInstanceNode, got %T", nodes[0])
	}
	if len(parentFolder.DBInstances) != 1 {
		t.Fatalf("expected 1 db instance in parent folder, got %d", len(parentFolder.DBInstances))
	}
}

func TestMutate_UpdateDbInstanceName(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	if err := g.Mutate(ctx, generated.MutationCommit{
		TableName: "db_instance",
		Operation: "insert",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1",
			"uri":"db-1",
			"name":"OldDB",
			"db_type":"postgres",
			"dsn":"postgres://user:pass@localhost:5432/db",
			"workspace_id":"ws-1",
			"folder_id":"root"
		}`,
	}); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	mut := generated.MutationCommit{
		TableName: "db_instance",
		Operation: "update",
		ObjectID:  "db-1",
		Payload: `{
			"name":"NewDB"
		}`,
	}

	if err := g.Mutate(ctx, mut); err != nil {
		t.Fatalf("mutate update failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	db := nodes[0].(*DBInstanceNode)
	if db.Name != "NewDB" {
		t.Errorf("expected Name=NewDB, got %s", db.Name)
	}
}

func TestMutate_DeleteDbInstance(t *testing.T) {
	ctx := context.Background()
	g := setupGraph()

	if err := g.Mutate(ctx, generated.MutationCommit{
		TableName: "db_instance",
		Operation: "insert",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1",
			"uri":"db-1",
			"name":"DBToDelete",
			"db_type":"sqlite",
			"dsn":"file:test.db",
			"workspace_id":"ws-1",
			"folder_id":"root"
		}`,
	}); err != nil {
		t.Fatalf("mutate insert failed: %v", err)
	}

	if err := g.Mutate(ctx, generated.MutationCommit{
		TableName: "db_instance",
		Operation: "delete",
		ObjectID:  "db-1",
		Payload: `{
			"id":"db-1"
		}`,
	}); err != nil {
		t.Fatalf("mutate delete failed: %v", err)
	}

	nodes := g.lookupAll([]string{"db-1"})
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after delete, got %d", len(nodes))
	}
}
