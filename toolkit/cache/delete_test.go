package cache

import (
	"sync"
	"testing"
	"time"
)

func TestOnDeleteFiresForEveryDeletionPath(t *testing.T) {
	t.Run("LRU overflow", func(t *testing.T) {
		var got []string
		c := New(Options{MaxEntries: 2, OnDelete: func(k string, v any) {
			got = append(got, k+"="+v.(string))
		}})
		c.Set("a", "1")
		c.Set("b", "2")
		c.Set("c", "3") // pushes out "a"
		if len(got) != 1 || got[0] != "a=1" {
			t.Fatalf("want [a=1], got %v", got)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		var got []string
		c := New(Options{OnDelete: func(k string, v any) { got = append(got, k) }})
		c.Set("a", 1)
		c.Delete("a")
		c.Delete("a") // already gone: must not fire twice
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("want [a], got %v", got)
		}
	})

	t.Run("DeleteFunc", func(t *testing.T) {
		var got []string
		c := New(Options{OnDelete: func(k string, v any) { got = append(got, k) }})
		c.Set("a", 1)
		c.Set("b", 2)
		c.DeleteFunc(func(string) bool { return true })
		if len(got) != 2 {
			t.Fatalf("want 2 deletions, got %v", got)
		}
	})

	t.Run("Set replacing a value", func(t *testing.T) {
		var got []any
		c := New(Options{OnDelete: func(k string, v any) { got = append(got, v) }})
		c.Set("a", "old")
		c.Set("a", "new")
		if len(got) != 1 || got[0] != "old" {
			t.Fatalf("want [old], got %v", got)
		}
	})

	t.Run("Set over an uncomparable value does not panic", func(t *testing.T) {
		// dumpCache in dialect/engine stores []byte and re-Sets the same key on
		// refresh. Comparing two uncomparable values panics, so a cache with no
		// OnDelete must never reach that comparison.
		c := New()
		c.Set("k", []byte("first"))
		c.Set("k", []byte("second"))
		if v, ok := c.Get("k"); !ok || string(v.([]byte)) != "second" {
			t.Fatalf("got %v, %v; want second, true", v, ok)
		}
	})

	t.Run("Set with an identical value does not delete", func(t *testing.T) {
		var got []any
		c := New(Options{OnDelete: func(k string, v any) { got = append(got, v) }})
		c.Set("a", "same")
		c.Set("a", "same")
		if len(got) != 0 {
			t.Fatalf("want no deletion, got %v", got)
		}
	})

	t.Run("TTL expiry observed through Get", func(t *testing.T) {
		var mu sync.Mutex
		var got []string
		c := New(Options{TTL: 40 * time.Millisecond, OnDelete: func(k string, v any) {
			mu.Lock()
			got = append(got, k)
			mu.Unlock()
		}})
		c.Set("a", 1)
		time.Sleep(80 * time.Millisecond)
		if _, ok := c.Get("a"); ok {
			t.Fatal("entry should have expired")
		}
		mu.Lock()
		defer mu.Unlock()
		if len(got) != 1 {
			t.Fatalf("want 1 deletion, got %v", got)
		}
	})

	t.Run("TTL expiry via the background sweeper", func(t *testing.T) {
		var mu sync.Mutex
		fired := make(chan string, 4)
		c := New(Options{TTL: 60 * time.Millisecond, OnDelete: func(k string, v any) {
			mu.Lock()
			defer mu.Unlock()
			fired <- k
		}})
		c.Set("a", 1)
		select {
		case k := <-fired:
			if k != "a" {
				t.Fatalf("deleted %q", k)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("sweeper never deleted the expired entry")
		}
	})
}

// TestGetRefreshesTTL documents the property the pool-close grace period relies
// on: an actively used entry is never deleted out from under its users.
func TestGetRefreshesTTL(t *testing.T) {
	c := New(Options{TTL: 100 * time.Millisecond})
	c.Set("a", 1)
	for i := 0; i < 5; i++ {
		time.Sleep(40 * time.Millisecond)
		if _, ok := c.Get("a"); !ok {
			t.Fatalf("entry expired after %d refreshes despite continuous use", i)
		}
	}
}

// TestOnDeleteRunsWithoutTheCacheLock: OnDelete does real work (closing a pool),
// so it must not run under the lock.
func TestOnDeleteRunsWithoutTheCacheLock(t *testing.T) {
	done := make(chan struct{})
	var c *Cache
	c = New(Options{MaxEntries: 1, OnDelete: func(k string, v any) {
		// Reading the cache from inside OnDelete would deadlock if the lock
		// were still held.
		_, _ = c.Get("b")
		close(done)
	}})
	c.Set("a", 1)
	c.Set("b", 2)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnDelete deadlocked: it runs while the cache lock is held")
	}
}

func TestOnDeleteConcurrent(t *testing.T) {
	var mu sync.Mutex
	count := 0
	c := New(Options{MaxEntries: 8, OnDelete: func(k string, v any) {
		mu.Lock()
		count++
		mu.Unlock()
	}})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 64; j++ {
				c.Set(string(rune('a'+i))+string(rune('0'+j%10)), j)
				c.Get(string(rune('a' + i)))
			}
		}(i)
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if count == 0 {
		t.Fatal("expected deletions under LRU pressure")
	}
}
