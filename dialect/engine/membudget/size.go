package membudget

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// defaultTotal is the budget where no memory cap is known.
	defaultTotal = 1 << 30

	// spikeReserve is kept out of the budget for the driver's copy of a value,
	// made before a statement can ask for room: sixteen statements at 16 MiB.
	spikeReserve = 16 * 16 << 20
)

// Size is the budget for this process and where it came from: MEMORY_BUDGET_MB,
// else about 80% of the cgroup's memory cap less the spike reserve, else a default.
func Size() (bytes int64, source string) {
	return sizeFrom("/proc/self/cgroup", "/sys/fs/cgroup")
}

func sizeFrom(procCgroup, root string) (int64, string) {
	if mb, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("MEMORY_BUDGET_MB")), 10, 64); err == nil && mb > 0 {
		return mb << 20, "MEMORY_BUDGET_MB"
	}
	if limit, ok := cgroupLimit(procCgroup, root); ok {
		return limit*8/10 - min(spikeReserve, limit/4), "the cgroup memory limit"
	}
	return defaultTotal, "the default, no memory limit found"
}

// cgroupLimit reads the memory cap of this process's cgroup (version 2). ok is
// false when the file is missing or the cap is "max".
func cgroupLimit(procCgroup, root string) (int64, bool) {
	raw, err := os.ReadFile(procCgroup)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		path, found := strings.CutPrefix(line, "0::")
		if !found {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, path, "memory.max"))
		if err != nil {
			return 0, false
		}
		limit, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		if err != nil || limit <= 0 {
			return 0, false
		}
		return limit, true
	}
	return 0, false
}
