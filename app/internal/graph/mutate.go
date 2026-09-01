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

	// WorkspaceGraph is an exported field, so a graph can arrive here having
	// been assigned rather than built. Index it before looking anything up.
	if g.index == nil {
		g.ensureIndex()
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
		g.detachByIDs([]string{commit.ObjectID})
	default:
		return fmt.Errorf("unknown operation: %s", commit.Operation)
	}

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

	// A folder the graph already holds keeps its node: the incoming one is an
	// empty shell built from the event, and swapping it in would drop the
	// folder's children and the record that it had been read from disk.
	if _, isFolder := node.(*FolderNode); isFolder {
		if _, exists := g.lookup(node.GetIDs()[0]).(*FolderNode); exists {
			return nil
		}
	}

	// Otherwise replace rather than duplicate: an insert for an ID the graph
	// already holds — a file restored by git, a db instance whose config was
	// rewritten — arrives as an insert, not an update.
	g.detachByIDs(node.GetIDs())
	g.attach(node)
	ensureNodeArrays(node)

	return nil
}

func (g *Graph) handleUpdate(objectID string, dto interface{}) error {
	for _, target := range g.lookupAll([]string{objectID}) {
		assignNonZero(target, dto)
		ensureNodeArrays(target)
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
