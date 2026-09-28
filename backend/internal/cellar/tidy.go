package cellar

import (
	"context"
	"log"
	"path/filepath"
	"syscall"
	"time"

	"github.com/selectDb/dialect/engine/connect"
)

// restAfter is how long a database replicates after its last use.
const restAfter = 15 * time.Minute

// minFreeShare is the share of the disk evict keeps free.
const minFreeShare = 0.2

// tidyEvery rests idle databases and evicts resting ones while the disk is
// short, until ctx ends.
func (d *Databases) tidyEvery(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
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
	idleIDs := []string{}
	for id, lastUsed := range d.lastUsed {
		if now.Sub(lastUsed) >= restAfter && d.store.FindDB(filepath.Join(d.dir, id+".db")) != nil {
			idleIDs = append(idleIDs, id)
		}
	}
	d.mu.Unlock()
	for _, id := range idleIDs {
		d.restOne(ctx, id, now)
	}
}

// restOne syncs outside the lock, which a sync to the bucket would hold for a
// round trip, then unregisters unless a Use came meanwhile.
func (d *Databases) restOne(ctx context.Context, id string, now time.Time) {
	path := filepath.Join(d.dir, id+".db")
	db := d.store.FindDB(path)
	if db == nil {
		return
	}
	// Unregistering syncs only a database Litestream has opened; this opens it.
	if err := db.SyncAndWait(ctx); err != nil {
		log.Printf("cellar: rest %s: %v", id, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if now.Sub(d.lastUsed[id]) < restAfter {
		return
	}
	connect.DeleteConnsByAddr(path)
	if err := d.store.UnregisterDB(ctx, path); err != nil {
		// Not in sync, so not safe to evict: replicate again, retry next time.
		log.Printf("cellar: rest %s: %v", id, err)
		_ = d.replicate(id)
	}
}

// evict deletes resting databases, least recently used first, while
// diskShort holds. Their replica holds them; the next Use restores them.
func (d *Databases) evict(diskShort func() bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for diskShort() {
		oldestID := ""
		for id, lastUsed := range d.lastUsed {
			resting := d.store.FindDB(filepath.Join(d.dir, id+".db")) == nil
			if resting && (oldestID == "" || lastUsed.Before(d.lastUsed[oldestID])) {
				oldestID = id
			}
		}
		if oldestID == "" {
			log.Printf("cellar: disk short and no resting database to evict")
			return
		}
		if err := removeDatabaseFiles(d.dir, oldestID); err != nil {
			log.Printf("cellar: evict %s: %v", oldestID, err)
			return
		}
		delete(d.lastUsed, oldestID)
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
