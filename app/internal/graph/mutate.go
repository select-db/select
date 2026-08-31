package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
	"time"

	"github.com/selectDb/toolkit"

	"selectDb/internal/desktop"
)

var nodeBuilders = map[string]func(interface{}) Node{
	"folder": func(src interface{}) Node {
		return BuildFolderNode(src.(FolderDTO))
	},
	"file": func(src interface{}) Node {
		return BuildFileNode(src.(FileDTO))
	},
	"db_instance": func(src interface{}) Node {
		return BuildDBInstanceNode(src.(DBInstanceDTO))
	},
}

func (g *Graph) Mutate(ctx context.Context, commit generated.MutationCommit) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.WorkspaceGraph == nil {
		return nil
	}

	toolkit.Debug("mutation", func() {
		log.Printf("[mutation] %+v", commit)
	})

	payloadBytes, err := toBytes(commit.Payload)
	if err != nil {
		toolkit.Debug("mutation", func() {
			log.Printf("[toBytes error] %v", err)
		})
		return err
	}

	dto, err := unmarshalDTO(payloadBytes, commit.TableName, commit.Operation)
	if err != nil {
		toolkit.Debug("mutation", func() {
			log.Printf("[unmarshalDTO error] %v", err)
		})
		return err
	}

	toolkit.Debug("mutation", func() {
		log.Printf("[DTO] %+v", dto)
	})

	switch commit.Operation {
	case "insert":
		if err := g.handleInsert(dto, commit.TableName); err != nil {
			return err
		}
	case "update":
		if err := g.handleUpdate(commit.ObjectID, dto); err != nil {
			return err
		}
	case "delete":
		RemoveNodesByIDs(g.WorkspaceGraph, []string{commit.ObjectID})
	default:
		return fmt.Errorf("unknown operation: %s", commit.Operation)
	}

	ensureArrays(g.WorkspaceGraph)

	commit.Payload = dto
	desktop.Emit("mutation", commit)
	utils.DebouncedEventsEmit("workspaceGraphUpdated", 100*time.Millisecond, g.WorkspaceGraph)

	return nil
}

func (g *Graph) handleInsert(dto interface{}, tableName string) error {
	builder, ok := nodeBuilders[tableName]
	if !ok {
		return fmt.Errorf("no node builder for table: %s", tableName)
	}

	node := builder(dto)

	parents := FindNodesByIds(g.WorkspaceGraph, node.GetParentIDs())
	for _, parent := range parents {
		RemoveNodesByIDs(parent, node.GetIDs())
		parent.AddChild(node)
	}
	return nil
}

func (g *Graph) handleUpdate(objectID string, dto interface{}) error {
	targets := FindNodesByIds(g.WorkspaceGraph, []string{objectID})

	for _, target := range targets {
		assignNonZero(target, dto)
	}

	return nil
}

func toBytes(payload interface{}) ([]byte, error) {
	switch v := payload.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case json.RawMessage:
		return []byte(v), nil
	case map[string]interface{}:
		return json.Marshal(v)
	default:
		// Fallback: JSON-encode arbitrary payloads (e.g. DTO structs) so that
		// unmarshalDTO can interpret them. This keeps Graph.Mutate flexible for
		// both database-originated mutations and synthetic ones from the
		// filesystem watcher.
		return json.Marshal(v)
	}
}

func unmarshalDTO(payloadBytes []byte, table string, operation string) (interface{}, error) {
	if operation == "delete" {
		return nil, nil
	}

	switch table {
	case "folder":
		var f FolderDTO
		if err := json.Unmarshal(payloadBytes, &f); err != nil {
			return nil, fmt.Errorf("failed to unmarshal folder: %w", err)
		}
		return f, nil
	case "file":
		var f FileDTO
		if err := json.Unmarshal(payloadBytes, &f); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file: %w", err)
		}
		return f, nil
	case "db_instance":
		var d DBInstanceDTO
		if err := json.Unmarshal(payloadBytes, &d); err != nil {
			return nil, fmt.Errorf("failed to unmarshal db_instance: %w", err)
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unknown table: %s", table)
	}
}
