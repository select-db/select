package cellar

import (
	"context"
	"log"
	"syscall"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/selectDb/dialect/engine/connect"
)

const (
	restAfter = 15 * time.Minute
	// minFreeShare is the share of the cellar's disk evict keeps free.
	minFreeShare = 0.2
	tidyInterval = time.Minute
)

// tidyEvery is the cellar's housekeeping: it rests idle databases and evicts
// resting ones while the cellar's disk is short, until ctx ends.
func (databases *Databases) tidyEvery(ctx context.Context) {
	defer close(databases.tidyStopped)
	ticker := time.NewTicker(tidyInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			databases.rest(ctx, now)
			databases.evict(func() bool { return freeShare(databases.dir) < minFreeShare })
		}
	}
}

// rest stops replicating the databases idle for restAfter, once the bucket
// holds every write: a resting database is whole in the bucket.
func (databases *Databases) rest(ctx context.Context, now time.Time) {
	databases.mu.Lock()
	idle := map[*database]*litestream.DB{}
	for _, database := range databases.onDisk {
		if database.replicating != nil && now.Sub(database.lastUsed) >= restAfter {
			idle[database] = database.replicating
		}
	}
	databases.mu.Unlock()
	for database, replicating := range idle {
		databases.restOne(ctx, database, replicating, now)
	}
}

// restOne syncs without the lock, as a sync to the bucket is a round trip,
// then unregisters unless the database was used or removed meanwhile.
func (databases *Databases) restOne(ctx context.Context, database *database, replicating *litestream.DB, now time.Time) {
	// Unregistering syncs only a database Litestream has opened; this opens it.
	if err := replicating.SyncAndWait(ctx); err != nil {
		log.Printf("cellar: rest %s: %v", database.id, err)
		return
	}
	databases.mu.Lock()
	defer databases.mu.Unlock()
	if databases.onDisk[database.id] != database || now.Sub(database.lastUsed) < restAfter {
		return
	}
	connect.DeleteConnsByAddr(database.path)
	database.replicating = nil
	// Not cancelled by Close: a half-done unregister would leave the database unsynced.
	if err := databases.store.UnregisterDB(context.WithoutCancel(ctx), database.path); err != nil {
		// Not in sync, so not safe to evict: replicate again, retry next time.
		log.Printf("cellar: rest %s: %v", database.id, err)
		_ = databases.replicate(database)
	}
}

// evict deletes resting databases from the cellar's disk, least recently used
// first, while diskShort holds. The bucket holds them; the next use restores
// them.
func (databases *Databases) evict(diskShort func() bool) {
	databases.mu.Lock()
	defer databases.mu.Unlock()
	for diskShort() {
		var oldest *database
		for _, database := range databases.onDisk {
			if database.replicating == nil && (oldest == nil || database.lastUsed.Before(oldest.lastUsed)) {
				oldest = database
			}
		}
		if oldest == nil {
			log.Printf("cellar: disk short and no resting database to evict")
			return
		}
		// Forgotten first: a half-removed database must be restored, not used.
		delete(databases.onDisk, oldest.id)
		if err := removeDatabaseFiles(oldest.path); err != nil {
			log.Printf("cellar: evict %s: %v", oldest.id, err)
			return
		}
	}
}

// freeShare is the share of dir's disk still free; 1 when it cannot tell, so a
// failed check never evicts.
func freeShare(dir string) float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil || stat.Blocks == 0 {
		return 1
	}
	return float64(stat.Bavail) / float64(stat.Blocks)
}
