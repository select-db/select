// Package membudget bounds the memory a server spends on streaming results.
// A statement takes a Lease and tops it up as its rows turn out wide; a top-up
// the budget cannot grant fails at once, so a statement never waits while holding memory.
package membudget

import (
	"errors"
	"sync"
)

const (
	// EntryCost is what every statement reserves to start.
	EntryCost = 2 << 20

	// Step is the granularity of a top-up.
	Step = 1 << 20
)

// ErrPressure is a statement refused, or ended, because the budget is spent.
var ErrPressure = errors.New("memory budget exhausted")

// Budget is a count of reserved bytes under Total. Top-ups stop at 90% of it,
// so the last tenth is left for statements to start in. A nil Budget has no limit.
type Budget struct {
	Total int64

	mu      sync.Mutex
	used    int64
	refused int64
}

// Begin starts a statement, reserving EntryCost.
func (b *Budget) Begin() (*Lease, error) {
	if b == nil {
		return nil, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+EntryCost > b.Total {
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
	return b.used, b.Total, b.refused
}

// Lease is one statement's reservation.
type Lease struct {
	budget *Budget
	held   int64
}

// Grow raises the reservation to at least target, rounded up to Step, or fails
// at once with ErrPressure and keeps what it had. A lease belongs to one goroutine.
func (l *Lease) Grow(target int64) error {
	if l == nil || target <= l.held {
		return nil
	}
	target = (target + Step - 1) / Step * Step
	b := l.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+target-l.held > b.Total-b.Total/10 {
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
