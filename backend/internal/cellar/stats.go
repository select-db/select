package cellar

import (
	"os"
	"sort"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/selectDb/toolkit/cache"
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
	// OldestSyncSeconds is the age of the last sync pass of the replicating database
	// that synced least recently; a growing age is a stuck replication.
	OldestSyncSeconds int64          `json:"oldestSyncSeconds"`
	NeverSynced       int            `json:"neverSynced"` // replicating, and no sync pass yet
	Largest           []DatabaseStat `json:"largest,omitempty"`
}

const (
	largestCount = 10
	// sizesTTL is how long the size of the files is reused: it takes a stat each.
	sizesTTL = 30 * time.Second
)

var sizes = cache.New(cache.Options{TTL: sizesTTL})

// sized is what the files of the databases take, and the largest of them.
type sized struct {
	bytes   int64
	largest []DatabaseStat
}

// Stats is read only when /debug/stats is: nothing is counted or measured otherwise.
func Stats() any {
	if databases == nil {
		return StatsReport{}
	}
	return databases.stats(time.Now())
}

// stored is a database of the cellar as it was when the stats were taken.
type stored struct {
	id, path    string
	replicating *litestream.DB
	lastUsed    time.Time
}

func (databases *Databases) stats(now time.Time) StatsReport {
	report := StatsReport{Wakes: databases.wakes.Load(), Evictions: databases.evictions.Load()}
	var all []stored
	databases.mu.Lock()
	for _, database := range databases.onDisk {
		all = append(all, stored{database.id, database.path, database.replicating, database.lastUsed})
	}
	databases.mu.Unlock()

	for _, database := range all {
		if database.replicating == nil {
			report.Resting++
			continue
		}
		report.Replicating++
		if synced := database.replicating.LastSuccessfulSyncAt(); synced.IsZero() {
			report.NeverSynced++
		} else if age := int64(now.Sub(synced).Seconds()); age > report.OldestSyncSeconds {
			report.OldestSyncSeconds = age
		}
	}

	value, _ := sizes.GetOrCreate("sizes", func() (any, error) { return measure(all, now), nil })
	files := value.(sized)
	report.Bytes, report.Largest = files.bytes, files.largest
	return report
}

func measure(all []stored, now time.Time) sized {
	var files sized
	var each []DatabaseStat
	for _, database := range all {
		info, err := os.Stat(database.path)
		if err != nil {
			continue
		}
		state := "resting"
		if database.replicating != nil {
			state = "replicating"
		}
		files.bytes += info.Size()
		each = append(each, DatabaseStat{ID: database.id, Bytes: info.Size(), State: state, IdleSeconds: int64(now.Sub(database.lastUsed).Seconds())})
	}
	sort.Slice(each, func(a, b int) bool { return each[a].Bytes > each[b].Bytes })
	files.largest = each[:min(len(each), largestCount)]
	return files
}
