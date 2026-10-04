// Package membudget bounds the memory a server spends on streaming results.
//
// A statement takes a Lease: a small entry cost to start, then top-ups as its
// rows turn out wide. A top-up the budget cannot grant fails at once, so a
// statement never waits while holding memory, which is how two of them
// deadlock. Sized under the process's memory cap, the budget makes the cap a
// limit the server enforces on itself instead of one the kernel enforces for it.
package membudget

import (
	"errors"
	"sync"
)

const (
	// EntryCost is what every statement reserves to start, so a trivial one is
	// never held up by wide ones.
	EntryCost = 2 << 20

	// Step is the granularity of a top-up.
	Step = 1 << 20
)

// ErrPressure is a statement refused, or ended, because the budget is spent.
var ErrPressure = errors.New("memory budget exhausted")

// Budget is a count of reserved bytes under a ceiling.
type Budget struct {
	mu      sync.Mutex
	total   int64
	used    int64
	refused int64
}

// New returns a budget of total bytes. Top-ups stop at 90% of it, so the last
// tenth is left for statements to start in.
func New(total int64) *Budget {
	return &Budget{total: total}
}

// Begin starts a statement, reserving EntryCost.
func (b *Budget) Begin() (*Lease, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+EntryCost > b.total {
		b.refused++
		return nil, ErrPressure
	}
	b.used += EntryCost
	return &Lease{budget: b, held: EntryCost}, nil
}

// Stats is the bytes reserved, the budget, and the statements refused so far.
func (b *Budget) Stats() (used, total, refused int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used, b.total, b.refused
}

// Lease is one statement's reservation.
type Lease struct {
	budget *Budget
	held   int64
}

// Grow raises the reservation to at least target, rounded up to Step. It fails
// at once with ErrPressure when the budget cannot grant it, and then holds what
// it had. A lease is used by one goroutine.
func (l *Lease) Grow(target int64) error {
	if target <= l.held {
		return nil
	}
	target = (target + Step - 1) / Step * Step
	b := l.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+target-l.held > b.total-b.total/10 {
		b.refused++
		return ErrPressure
	}
	b.used += target - l.held
	l.held = target
	return nil
}

// Release returns the whole reservation. It is safe to call twice.
func (l *Lease) Release() {
	if l == nil || l.held == 0 {
		return
	}
	b := l.budget
	b.mu.Lock()
	b.used -= l.held
	b.mu.Unlock()
	l.held = 0
}
