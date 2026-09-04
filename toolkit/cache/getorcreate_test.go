package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGetOrCreateRunsCreateOnce is the point of the API: concurrent callers that
// all miss must produce one create call and one shared value, not one each.
func TestGetOrCreateRunsCreateOnce(t *testing.T) {
	c := New()

	var calls atomic.Int64
	create := func() (any, error) {
		calls.Add(1)
		// Wide enough that every goroutine is inside GetOrCreate before the
		// first one publishes.
		time.Sleep(50 * time.Millisecond)
		return "the-one", nil
	}

	const n = 32
	got := make([]any, n)
	errs := make([]error, n)
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(n)
	for i := range n {
		go func() {
			defer done.Done()
			start.Wait()
			got[i], errs[i] = c.GetOrCreate("k", create)
		}()
	}
	start.Done()
	done.Wait()

	if calls.Load() != 1 {
		t.Fatalf("create ran %d times, want 1", calls.Load())
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if got[i] != "the-one" {
			t.Fatalf("caller %d got %v, want the value from the single create", i, got[i])
		}
	}
}

func TestGetOrCreateStoresTheValue(t *testing.T) {
	c := New()

	if _, err := c.GetOrCreate("k", func() (any, error) { return 1, nil }); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Get("k"); !ok || v != 1 {
		t.Fatalf("Get after GetOrCreate = %v, %v; want 1, true", v, ok)
	}
	// A second call hits the cache rather than creating again.
	v, err := c.GetOrCreate("k", func() (any, error) { t.Fatal("create ran on a hit"); return nil, nil })
	if err != nil || v != 1 {
		t.Fatalf("GetOrCreate on a hit = %v, %v; want 1, nil", v, err)
	}
}

// TestGetOrCreateSharesTheErrorButDoesNotCacheIt: everyone waiting on a failed
// create sees the failure, and the next caller gets a fresh attempt.
func TestGetOrCreateSharesTheErrorButDoesNotCacheIt(t *testing.T) {
	c := New()
	boom := errors.New("dial failed")

	var calls atomic.Int64
	failOnce := func() (any, error) {
		if calls.Add(1) == 1 {
			time.Sleep(50 * time.Millisecond)
			return nil, boom
		}
		return "recovered", nil
	}

	const n = 8
	errs := make([]error, n)
	var done sync.WaitGroup
	done.Add(n)
	for i := range n {
		go func() {
			defer done.Done()
			_, errs[i] = c.GetOrCreate("k", failOnce)
		}()
	}
	done.Wait()

	for i := range n {
		if !errors.Is(errs[i], boom) {
			t.Fatalf("caller %d got %v, want the create error", i, errs[i])
		}
	}
	if _, ok := c.Get("k"); ok {
		t.Fatal("a failed create was cached")
	}

	v, err := c.GetOrCreate("k", failOnce)
	if err != nil || v != "recovered" {
		t.Fatalf("retry after failure = %v, %v; want recovered, nil", v, err)
	}
}

// TestGetOrCreateIsPerKey: a slow create for one key must not block another.
func TestGetOrCreateIsPerKey(t *testing.T) {
	c := New()

	release := make(chan struct{})
	slowStarted := make(chan struct{})
	go func() {
		_, _ = c.GetOrCreate("slow", func() (any, error) {
			close(slowStarted)
			<-release
			return "slow", nil
		})
	}()
	<-slowStarted

	fast := make(chan any, 1)
	go func() {
		v, _ := c.GetOrCreate("fast", func() (any, error) { return "fast", nil })
		fast <- v
	}()

	select {
	case v := <-fast:
		if v != "fast" {
			t.Fatalf("got %v, want fast", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a create for one key blocked a create for another")
	}
	close(release)
}

// TestGetOrCreateSurvivesAPanickingCreate: a panic must not leave the key
// permanently held, which would wedge every later caller for it.
func TestGetOrCreateSurvivesAPanickingCreate(t *testing.T) {
	c := New()

	waiterErr := make(chan error, 1)
	panicking := make(chan struct{})
	release := make(chan struct{})

	go func() {
		defer func() { _ = recover() }()
		_, _ = c.GetOrCreate("k", func() (any, error) {
			close(panicking)
			<-release
			panic("boom")
		})
	}()
	<-panicking

	go func() {
		v, err := c.GetOrCreate("k", func() (any, error) { return nil, errors.New("unreachable") })
		_ = v
		waiterErr <- err
	}()
	// Let the waiter attach to the in-flight call before the panic unwinds.
	time.Sleep(50 * time.Millisecond)
	close(release)

	select {
	case err := <-waiterErr:
		if err == nil {
			t.Fatal("waiter on a panicking create got no error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter was never released after create panicked")
	}

	// The key is usable again.
	done := make(chan struct{})
	go func() {
		defer close(done)
		v, err := c.GetOrCreate("k", func() (any, error) { return "after", nil })
		if err != nil || v != "after" {
			t.Errorf("after a panic: %v, %v; want after, nil", v, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the key stayed wedged after create panicked")
	}
}

// TestGetOrCreateFiresOnRemoveForAnExpiredEntry: a miss caused by expiry is
// still a removal, so the old value must be handed to OnRemove before the new
// one is created.
func TestGetOrCreateFiresOnRemoveForAnExpiredEntry(t *testing.T) {
	var mu sync.Mutex
	var removed []any
	c := New(Options{TTL: 40 * time.Millisecond, OnRemove: func(_ string, v any) {
		mu.Lock()
		removed = append(removed, v)
		mu.Unlock()
	}})

	c.Set("k", "old")
	time.Sleep(80 * time.Millisecond)

	v, err := c.GetOrCreate("k", func() (any, error) { return "new", nil })
	if err != nil || v != "new" {
		t.Fatalf("GetOrCreate after expiry = %v, %v; want new, nil", v, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(removed) != 1 || removed[0] != "old" {
		t.Fatalf("OnRemove saw %v, want [old]", removed)
	}
}

// TestGetOrCreateEvictsUnderMaxEntries: values it stores are ordinary entries,
// subject to the same LRU bound as Set.
func TestGetOrCreateEvictsUnderMaxEntries(t *testing.T) {
	var mu sync.Mutex
	var removed []string
	c := New(Options{MaxEntries: 2, OnRemove: func(k string, _ any) {
		mu.Lock()
		removed = append(removed, k)
		mu.Unlock()
	}})

	for _, k := range []string{"a", "b", "c"} {
		if _, err := c.GetOrCreate(k, func() (any, error) { return k, nil }); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(removed) != 1 || removed[0] != "a" {
		t.Fatalf("evicted %v, want [a]", removed)
	}
}
