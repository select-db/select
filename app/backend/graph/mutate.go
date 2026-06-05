package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
	"time"

	"github.com/selectDb/toolkit"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

var eventsEmit = runtime.EventsEmit

// debouncedEventsEmit is a package-level indirection so tests can replace it
// with a no-op; the production utils.DebouncedEventsEmit schedules a wails
// runtime emit on a timer, which calls log.Fatal when invoked with a
// non-lifecycle context (as is the case in unit tests).
var debouncedEventsEmit = utils.DebouncedEventsEmit

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

	commit.Payload = dto
	eventsEmit(ctx, "mutation", commit)
	debouncedEventsEmit(ctx, "workspaceGraphUpdated", 100*time.Millisecond, g.WorkspaceGraph)

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

	FindNodesByIds(g.WorkspaceGraph, []string{objectID})

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
