package cellar

import (
	"os"
	"sort"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
)

// DatabaseStat is one of the databases that take the most room on the cellar's disk.
type DatabaseStat struct {
	ID          string `json:"id"`
	Bytes       int64  `json:"bytes"`
	State       string `json:"state"` // replicating or resting
	IdleSeconds int64  `json:"idleSeconds"`
}

// StatsReport is the "cellar" section of /debug/stats.
type StatsReport struct {
	Replicating int   `json:"replicating"` // used within restAfter: every write streams to the bucket
	Resting     int   `json:"resting"`     // on disk, whole in the bucket
	Bytes       int64 `json:"bytes"`       // what they take on the cellar's disk
	Wakes       int64 `json:"wakes"`       // databases restored from the bucket since the start
	Evictions   int64 `json:"evictions"`   // databases removed from the disk since the start
	// OldestSyncSeconds is how long ago the replicating database that synced least
	// recently did, and NeverSynced how many have not yet: a growing age is a
	// replication that is stuck, a database whose writes the bucket does not have.
	OldestSyncSeconds int64          `json:"oldestSyncSeconds"`
	NeverSynced       int            `json:"neverSynced"`
	Largest           []DatabaseStat `json:"largest,omitempty"`
}

// statsTTL is how long the size of the files is reused: it takes a stat per database.
const (
	statsTTL     = 30 * time.Second
	largestCount = 10
)

var (
	sizeMu      sync.Mutex
	sizeAt      time.Time
	sizeBytes   int64
	sizeLargest []DatabaseStat
)

// Stats is read only when /debug/stats is, so nothing is counted while nobody looks.
// The states, the counters and the age of the syncs cost a pass over a map; the size
// of the files, a stat each, is read at most every statsTTL.
func Stats() any {
	if databases == nil {
		return StatsReport{}
	}
	return databases.stats(time.Now())
}

func (databases *Databases) stats(now time.Time) StatsReport {
	report := StatsReport{Wakes: databases.wakes.Load(), Evictions: databases.evictions.Load()}
	type seen struct {
		id, path    string
		replicating *litestream.DB
		lastUsed    time.Time
	}
	var all []seen
	databases.mu.Lock()
	for _, database := range databases.onDisk {
		all = append(all, seen{database.id, database.path, database.replicating, database.lastUsed})
	}
	databases.mu.Unlock()

	for _, d := range all {
		if d.replicating == nil {
			report.Resting++
			continue
		}
		report.Replicating++
		if synced := d.replicating.LastSuccessfulSyncAt(); synced.IsZero() {
			report.NeverSynced++
		} else if age := int64(now.Sub(synced).Seconds()); age > report.OldestSyncSeconds {
			report.OldestSyncSeconds = age
		}
	}

	sizeMu.Lock()
	defer sizeMu.Unlock()
	if now.Sub(sizeAt) >= statsTTL {
		sizeBytes, sizeLargest = 0, nil
		var sized []DatabaseStat
		for _, d := range all {
			info, err := os.Stat(d.path)
			if err != nil {
				continue
			}
			state := "resting"
			if d.replicating != nil {
				state = "replicating"
			}
			sizeBytes += info.Size()
			sized = append(sized, DatabaseStat{ID: d.id, Bytes: info.Size(), State: state, IdleSeconds: int64(now.Sub(d.lastUsed).Seconds())})
		}
		sort.Slice(sized, func(a, b int) bool { return sized[a].Bytes > sized[b].Bytes })
		sizeLargest = sized[:min(len(sized), largestCount)]
		sizeAt = now
	}
	report.Bytes = sizeBytes
	report.Largest = sizeLargest
	return report
}
