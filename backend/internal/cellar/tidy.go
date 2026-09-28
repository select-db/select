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
	// restAfter is how long a database replicates after its last use.
	restAfter = 15 * time.Minute
	// minFreeShare is the share of the disk evict keeps free.
	minFreeShare = 0.2
	tidyInterval = time.Minute
)

// tidyEvery rests idle databases and evicts resting ones while the disk is
// short, until ctx ends.
func (d *Databases) tidyEvery(ctx context.Context) {
	ticker := time.NewTicker(tidyInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			d.rest(ctx, now)
			d.evict(func() bool { return freeShare(d.dir) < minFreeShare })
		}
	}
}

// rest stops replicating the databases idle for restAfter, once the replica
// holds every write: a resting database is whole in its replica.
func (d *Databases) rest(ctx context.Context, now time.Time) {
	d.mu.Lock()
	idle := map[*database]*litestream.DB{}
	for _, database := range d.onDisk {
		if database.replicating != nil && now.Sub(database.lastUsed) >= restAfter {
			idle[database] = database.replicating
		}
	}
	d.mu.Unlock()
	for database, replicating := range idle {
		d.restOne(ctx, database, replicating, now)
	}
}

// restOne syncs without the lock, as a sync to the bucket is a round trip,
// then unregisters unless the database was used or removed meanwhile.
func (d *Databases) restOne(ctx context.Context, database *database, replicating *litestream.DB, now time.Time) {
	// Unregistering syncs only a database Litestream has opened; this opens it.
	if err := replicating.SyncAndWait(ctx); err != nil {
		log.Printf("cellar: rest %s: %v", database.id, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.onDisk[database.id] != database || now.Sub(database.lastUsed) < restAfter {
		return
	}
	connect.DeleteConnsByAddr(database.path)
	database.replicating = nil
	if err := d.store.UnregisterDB(ctx, database.path); err != nil {
		// Not in sync, so not safe to evict: replicate again, retry next time.
		log.Printf("cellar: rest %s: %v", database.id, err)
		_ = d.replicate(database)
	}
}

// evict deletes resting databases, least recently used first, while
// diskShort holds. Their replica holds them; the next use restores them.
func (d *Databases) evict(diskShort func() bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for diskShort() {
		var oldest *database
		for _, database := range d.onDisk {
			if database.replicating == nil && (oldest == nil || database.lastUsed.Before(oldest.lastUsed)) {
				oldest = database
			}
		}
		if oldest == nil {
			log.Printf("cellar: disk short and no resting database to evict")
			return
		}
		if err := removeDatabaseFiles(oldest.path); err != nil {
			log.Printf("cellar: evict %s: %v", oldest.id, err)
			return
		}
		delete(d.onDisk, oldest.id)
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
