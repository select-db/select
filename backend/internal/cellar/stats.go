package cellar

import (
	"os"
	"sync"
	"time"
)

// StatsReport is the "cellar" section of /debug/stats.
type StatsReport struct {
	Replicating int   `json:"replicating"` // used within restAfter: every write streams to the bucket
	Resting     int   `json:"resting"`     // on disk, whole in the bucket
	Bytes       int64 `json:"bytes"`       // what they take on the cellar's disk
	Wakes       int64 `json:"wakes"`       // databases restored from the bucket since the start
	Evictions   int64 `json:"evictions"`   // databases removed from the disk since the start
}

// statsTTL is how long the size of the files is reused: it takes a stat per database.
const statsTTL = 30 * time.Second

var (
	sizeMu    sync.Mutex
	sizeAt    time.Time
	sizeBytes int64
)

// Stats is read only when /debug/stats is, so nothing is counted while nobody looks.
// The states and the counters cost a pass over a map; the size of the files, a stat
// each, is read at most every statsTTL.
func Stats() any {
	if databases == nil {
		return StatsReport{}
	}
	return databases.stats(time.Now())
}

func (databases *Databases) stats(now time.Time) StatsReport {
	report := StatsReport{Wakes: databases.wakes.Load(), Evictions: databases.evictions.Load()}
	var paths []string
	databases.mu.Lock()
	for _, database := range databases.onDisk {
		if database.replicating != nil {
			report.Replicating++
		} else {
			report.Resting++
		}
		paths = append(paths, database.path)
	}
	databases.mu.Unlock()

	sizeMu.Lock()
	defer sizeMu.Unlock()
	if now.Sub(sizeAt) >= statsTTL {
		var total int64
		for _, path := range paths {
			if info, err := os.Stat(path); err == nil {
				total += info.Size()
			}
		}
		sizeBytes, sizeAt = total, now
	}
	report.Bytes = sizeBytes
	return report
}
