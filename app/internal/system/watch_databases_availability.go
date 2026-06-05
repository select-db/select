package system

import (
	"context"
	"sync"
	"time"

	"selectDb/internal/db_client"
	"selectDb/internal/graph"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	pingBaseInterval = 20 * time.Second
	pingMaxInterval  = 240 * time.Second
)

// Stops any running DB availability watcher and starts a new one.
func (s *System) StartDatabaseWatcher() {
	s.mu.Lock()
	if s.dbWatcherCancel != nil {
		s.dbWatcherCancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.dbWatcherCancel = cancel
	s.mu.Unlock()

	go s.watchDatabases(ctx)
}

type pingResult struct {
	ID    string
	Error string
}

func (s *System) watchDatabases(ctx context.Context) {
	backoff := make(map[string]time.Duration)
	lastPing := make(map[string]time.Time)

	for {
		ws, err := s.Graph.GetWorkspaceGraph()
		if err == nil && ws != nil {
			dbs := s.Graph.WorkspaceGraph.DBInstances
			now := time.Now()

			var toCheck []*graph.DBInstanceNode
			for _, db := range dbs {
				interval := backoff[db.ID]
				if interval == 0 {
					interval = pingBaseInterval
				}
				if now.Sub(lastPing[db.ID]) >= interval {
					toCheck = append(toCheck, db)
				}
			}

			if len(toCheck) > 0 {
				results := make([]pingResult, len(toCheck))
				var wg sync.WaitGroup

				for i, db := range toCheck {
					wg.Add(1)
					go func(idx int, db *graph.DBInstanceNode) {
						defer wg.Done()
						result := s.DbClient.Ping(db_client.PingParams{
							DbInstanceID: db.ID,
							DbType:       db.DBType,
							Dsn:          db.DSN,
							FolderId:     db.FolderID,
							Ssh:          db.SSH,
							Proxified:    db.Proxified,
						})
						results[idx] = pingResult{ID: db.ID, Error: result}
					}(i, db)
				}
				wg.Wait()

				for _, r := range results {
					lastPing[r.ID] = now
					if r.Error != "" {
						b := backoff[r.ID]
						if b == 0 {
							b = pingBaseInterval
						}
						b *= 2
						if b > pingMaxInterval {
							b = pingMaxInterval
						}
						backoff[r.ID] = b
					} else {
						delete(backoff, r.ID)
					}
				}

				databases := make([]map[string]interface{}, 0, len(results))
				for _, r := range results {
					entry := map[string]interface{}{"id": r.ID}
					if r.Error != "" {
						entry["error"] = r.Error
					}
					databases = append(databases, entry)
				}
				runtime.EventsEmit(s.ctx, "databaseAvailability", map[string]interface{}{
					"databases": databases,
				})
			}

			// Clean up entries for removed databases
			dbIDs := make(map[string]bool, len(dbs))
			for _, db := range dbs {
				dbIDs[db.ID] = true
			}
			for id := range backoff {
				if !dbIDs[id] {
					delete(backoff, id)
					delete(lastPing, id)
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
