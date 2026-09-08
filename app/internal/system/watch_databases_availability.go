package system

import (
	"context"
	"sync"
	"time"

	"selectDb/internal/db_client"
	"selectDb/internal/graph"
)

const (
	pingBaseInterval = 20 * time.Second
	pingMaxInterval  = 240 * time.Second
)

// Stops any running DB availability watcher and starts a new one.
//
// The watcher only decides when to ping. What each ping found reaches the
// frontend from Ping itself, along with every other way a database is reached,
// so the dot moves the moment anything learns something rather than at the top
// of the next sweep.
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
				failures := make([]string, len(toCheck))
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
						failures[idx] = result
					}(i, db)
				}
				wg.Wait()

				for i, db := range toCheck {
					lastPing[db.ID] = now
					if failures[i] == "" {
						delete(backoff, db.ID)
						continue
					}

					b := backoff[db.ID]
					if b == 0 {
						b = pingBaseInterval
					}
					b *= 2
					if b > pingMaxInterval {
						b = pingMaxInterval
					}
					backoff[db.ID] = b
				}
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
