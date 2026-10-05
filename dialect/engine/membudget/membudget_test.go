package membudget

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestBeginReservesTheEntryCostAndReleaseReturnsIt(t *testing.T) {
	b := &Budget{Total: 100 << 20}
	lease, err := b.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if used, _, _ := b.Stats(); used != EntryCost {
		t.Fatalf("used %d, want the entry cost %d", used, EntryCost)
	}
	lease.Release()
	lease.Release()
	if used, _, _ := b.Stats(); used != 0 {
		t.Fatalf("used %d after release, want 0 (and a second release changes nothing)", used)
	}
}

func TestGrowRaisesTheReservationInStepsAndNeverLowersIt(t *testing.T) {
	b := &Budget{Total: 100 << 20}
	lease, _ := b.Begin()
	if err := lease.Grow(5<<20 + 1); err != nil {
		t.Fatal(err)
	}
	if used, _, _ := b.Stats(); used != 6<<20 {
		t.Fatalf("used %d MiB, want 6 (rounded up to a step)", used>>20)
	}
	if err := lease.Grow(1 << 20); err != nil {
		t.Fatal(err)
	}
	if used, _, _ := b.Stats(); used != 6<<20 {
		t.Fatalf("a smaller target must not lower the reservation, used %d MiB", used>>20)
	}
	lease.Release()
}

func TestGrowFailsAtOnceWhenTheBudgetCannotGrantIt(t *testing.T) {
	b := &Budget{Total: 100 << 20}
	lease, _ := b.Begin()
	defer lease.Release()
	if err := lease.Grow(95 << 20); !errors.Is(err, ErrPressure) {
		t.Fatalf("want ErrPressure above the 90%% ceiling, got %v", err)
	}
	if used, _, refused := b.Stats(); used != EntryCost || refused != 1 {
		t.Fatalf("a refused top-up keeps what the lease had: used %d, refused %d", used, refused)
	}
	if err := lease.Grow(80 << 20); err != nil {
		t.Fatalf("a top-up under the ceiling is granted: %v", err)
	}
}

func TestStatementsCanStartWhenWideOnesHaveTakenTheCeiling(t *testing.T) {
	b := &Budget{Total: 100 << 20}
	wide, _ := b.Begin()
	defer wide.Release()
	if err := wide.Grow(90 << 20); err != nil {
		t.Fatal(err)
	}
	small, err := b.Begin()
	if err != nil {
		t.Fatalf("the last tenth is for starting statements: %v", err)
	}
	small.Release()
}

func TestBeginIsRefusedOnceTheBudgetIsFull(t *testing.T) {
	b := &Budget{Total: 3 * EntryCost}
	var leases []*Lease
	for i := 0; i < 3; i++ {
		l, err := b.Begin()
		if err != nil {
			t.Fatal(err)
		}
		leases = append(leases, l)
	}
	if _, err := b.Begin(); !errors.Is(err, ErrPressure) {
		t.Fatalf("want ErrPressure, got %v", err)
	}
	leases[0].Release()
	if _, err := b.Begin(); err != nil {
		t.Fatalf("a released lease frees room: %v", err)
	}
}

func TestTheBudgetIsNeverExceededUnderConcurrency(t *testing.T) {
	const total = 64 << 20
	b := &Budget{Total: total}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := b.Begin()
			if err != nil {
				return
			}
			defer lease.Release()
			for target := int64(4 << 20); target <= 40<<20; target += 6 << 20 {
				if used, _, _ := b.Stats(); used > total {
					t.Errorf("used %d exceeds the budget %d", used, int64(total))
				}
				if lease.Grow(target) != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	if used, _, _ := b.Stats(); used != 0 {
		t.Fatalf("every lease released, used %d", used)
	}
}

func TestCgroupLimit(t *testing.T) {
	dir := t.TempDir()
	proc := filepath.Join(dir, "cgroup")
	root := filepath.Join(dir, "sys")
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(proc, "0::/system.slice/select-cellar.service\n")
	write(filepath.Join(root, "system.slice/select-cellar.service/memory.max"), "1073741824\n")
	if limit, ok := cgroupLimit(proc, root); !ok || limit != 1<<30 {
		t.Fatalf("got %d, %v; want 1 GiB", limit, ok)
	}
	write(filepath.Join(root, "system.slice/select-cellar.service/memory.max"), "max\n")
	if _, ok := cgroupLimit(proc, root); ok {
		t.Fatal("max is no cap")
	}
	if _, ok := cgroupLimit(filepath.Join(dir, "missing"), root); ok {
		t.Fatal("no cgroup file is no cap")
	}
}

func TestSizeLeavesRoomUnderTheCap(t *testing.T) {
	t.Setenv("MEMORY_BUDGET_MB", "")
	dir := t.TempDir()
	proc := filepath.Join(dir, "cgroup")
	if err := os.WriteFile(proc, []byte("0::/unit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "unit"), 0o755); err != nil {
		t.Fatal(err)
	}
	size := func(limit string) (int64, string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "unit", "memory.max"), []byte(limit), 0o644); err != nil {
			t.Fatal(err)
		}
		return sizeFrom(proc, dir)
	}
	for limit, wantMiB := range map[string]int64{"1073741824": 563, "3221225472": 2201} {
		if got, source := size(limit); got>>20 != wantMiB || source != "the cgroup memory limit" {
			t.Fatalf("cap %s: budget %d MiB from %q, want %d MiB from the cgroup", limit, got>>20, source, wantMiB)
		}
	}
	if got, _ := size("268435456"); got <= 0 || got > 154<<20 {
		t.Fatalf("a small cap keeps a quarter for spikes, budget %d MiB", got>>20)
	}
	if got, source := size("max"); got != defaultTotal || source == "the cgroup memory limit" {
		t.Fatalf("no cap falls back to the default and says so, got %d MiB from %q", got>>20, source)
	}
	t.Setenv("MEMORY_BUDGET_MB", "300")
	if got, source := size("1073741824"); got != 300<<20 || source != "MEMORY_BUDGET_MB" {
		t.Fatalf("the override wins, got %d MiB from %q", got>>20, source)
	}
}

func TestANilBudgetImposesNoLimit(t *testing.T) {
	var b *Budget
	lease, err := b.Begin()
	if err != nil || lease.Grow(1<<40) != nil {
		t.Fatalf("no budget, no limit: %v", err)
	}
	lease.Release()
}
