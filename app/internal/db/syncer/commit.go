package syncer

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

type QueueMutationParams struct {
	Query     string              `json:"query"`
	TableName string              `json:"table_name"`
	Operation string              `json:"operation"`
	Args      []driver.NamedValue `json:"args"`
}

// CommitMutation queues or merges a mutation into the sync queue.
func (s *Syncer) CommitMutation(params *QueueMutationParams) error {
	query, tableName, operation, args := params.Query, params.TableName, params.Operation, params.Args

	if strings.Contains(query, "@no-track") {
		return nil
	}

	if !isTableTrackable(tableName) {
		return nil
	}

	objectID, err := extractObjectID(args)
	if err != nil {
		return fmt.Errorf("failed to extract ID: %w", err)
	}

	payload, err := buildAndMergeArgs(args, nil)
	if err != nil {
		return fmt.Errorf("failed to marshal args for new mutation: %w", err)
	}

	ctx := context.Background()
	currentWTU, err := s.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			// Backend creates first workspace + workspace_to_user on signup
			// app pulls them via Sync, nothing to enqueue
			return nil
		}
		return fmt.Errorf("failed to get current workspace-to-user: %w", err)
	}

	// Look for existing queued mutation
	previousCommit, err := s.Queries.GetPreviousCommit(ctx, generated.GetPreviousCommitParams{
		Operation:   operation,
		TableName:   tableName,
		ObjectID:    objectID,
		UserID:      currentWTU.UserID,
		WorkspaceID: currentWTU.WorkspaceID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get similar mutation: %w", err)
	}

	if err == sql.ErrNoRows {
		// New commit
		_, err := s.Queries.SaveCommit(ctx, generated.SaveCommitParams{
			ID:          utils.GenerateUUID(),
			Operation:   operation,
			TableName:   tableName,
			ObjectID:    objectID,
			UserID:      currentWTU.UserID,
			WorkspaceID: currentWTU.WorkspaceID,
			Payload:     payload,
		})
		if err != nil {
			return fmt.Errorf("failed to queue mutation: %w", err)
		}
	} else {
		// Update existing commit
		existingBytes, err := utils.BytesFromInterface(previousCommit.Payload)
		if err != nil {
			return fmt.Errorf("invalid existing args type: %w", err)
		}

		mergedPayload, err := buildAndMergeArgs(args, existingBytes)
		if err != nil {
			return fmt.Errorf("failed to merge args: %w", err)
		}

		_, err = s.Queries.UpdateCommit(ctx, generated.UpdateCommitParams{
			ID:      previousCommit.ID,
			Payload: mergedPayload,
		})
		if err != nil {
			return fmt.Errorf("failed to update queued mutation: %w", err)
		}
	}

	_ = s.pushCommitToFrontend(generated.MutationCommit{
		Operation: operation,
		TableName: tableName,
		ObjectID:  objectID,
		Payload:   payload,
	})

	s.debouncedSync(currentWTU.UserID)
	return nil
}

// debouncedSync schedules a single Sync after 100ms of no further commits for this user.
func (s *Syncer) debouncedSync(userID string) {
	s.debounceMu.Lock()
	defer s.debounceMu.Unlock()

	s.debounceUserID = userID
	if s.debounceTimer != nil {
		s.debounceTimer.Reset(syncDebounceDelay)
		return
	}
	s.debounceTimer = time.AfterFunc(syncDebounceDelay, func() {
		s.debounceMu.Lock()
		uid := s.debounceUserID
		s.debounceUserID = ""
		s.debounceTimer = nil
		s.debounceMu.Unlock()
		if uid != "" {
			_ = s.Sync(context.Background(), uid)
		}
	})
}

var trackableTablesMap = map[string]struct{}{
	"workspace":         {},
	"workspace_to_user": {},
	"history":           {},
	"role":              {},
	"user_to_role":      {},
	"permission":        {},
}

func isTableTrackable(table string) bool {
	_, ok := trackableTablesMap[table]
	return ok
}

// extractObjectID find the "id" in args.
func extractObjectID(args []driver.NamedValue) (string, error) {
	var id string

	for _, arg := range args {
		if arg.Name == "id" {
			val, ok := arg.Value.(string)
			if !ok {
				return "", fmt.Errorf("id is not a string (got %T)", arg.Value)
			}
			id = val
		}
	}

	if id == "" {
		return "", fmt.Errorf("id not found in args")
	}

	return id, nil
}

// buildAndMergeArgs merges new named args into existing JSON-encoded args.
func buildAndMergeArgs(newArgs []driver.NamedValue, existing []byte) ([]byte, error) {
	merged := make(map[string]interface{})

	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &merged); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existing args: %w", err)
		}
	}

	for _, arg := range newArgs {
		if arg.Name == "" {
			return nil, fmt.Errorf("named value has no name: %+v", arg)
		}
		merged[arg.Name] = arg.Value
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged args to JSON: %w", err)
	}

	return data, nil
}
