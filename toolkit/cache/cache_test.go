package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New()
	c.Set("key", "value")
	v, ok := c.Get("key")
	if !ok || v != "value" {
		t.Fatalf("expected value, got %v %v", v, ok)
	}
}

func TestGetMiss(t *testing.T) {
	c := New()
	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestSetOverwrite(t *testing.T) {
	c := New()
	c.Set("k", "first")
	c.Set("k", "second")
	v, ok := c.Get("k")
	if !ok || v != "second" {
		t.Fatalf("expected second, got %v", v)
	}
}

func TestDelete(t *testing.T) {
	c := New()
	c.Set("k", "v")
	c.Delete("k")
	_, ok := c.Get("k")
	if ok {
		t.Fatal("expected miss after delete")
	}
}

func TestDeleteFunc(t *testing.T) {
	c := New()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.DeleteFunc(func(key string) bool { return key != "b" })
	if _, ok := c.Get("a"); ok {
		t.Error("a should be deleted")
	}
	if _, ok := c.Get("c"); ok {
		t.Error("c should be deleted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("b should survive")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := New(Options{TTL: 50 * time.Millisecond})
	c.Set("k", "v")
	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("k")
	if ok {
		t.Fatal("entry should have expired")
	}
}

func TestTTLReset(t *testing.T) {
	c := New(Options{TTL: 100 * time.Millisecond})
	c.Set("k", "v")

	// access just before expiry to reset TTL
	time.Sleep(70 * time.Millisecond)
	_, ok := c.Get("k")
	if !ok {
		t.Fatal("entry should still be alive")
	}

	// TTL was reset on Get; sleep another 70ms, total 140ms but only 70ms since last access
	time.Sleep(70 * time.Millisecond)
	_, ok = c.Get("k")
	if !ok {
		t.Fatal("entry should still be alive after TTL reset")
	}
}

func TestMaxEntriesEvictsLRU(t *testing.T) {
	c := New(Options{MaxEntries: 3})
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// access a and b to make c the LRU
	c.Get("a")
	c.Get("b")

	// adding d should evict c (least recently used)
	c.Set("d", 4)

	if _, ok := c.Get("c"); ok {
		t.Error("c should have been evicted")
	}
	for _, k := range []string{"a", "b", "d"} {
		if _, ok := c.Get(k); !ok {
			t.Errorf("%s should still be present", k)
		}
	}
}

func TestMaxEntriesExact(t *testing.T) {
	c := New(Options{MaxEntries: 2})
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts a (LRU)
	if _, ok := c.Get("a"); ok {
		t.Error("a should have been evicted")
	}
	if l := c.lru.Len(); l != 2 {
		t.Errorf("expected 2 entries, got %d", l)
	}
}

func TestLRUOrderAfterGet(t *testing.T) {
	c := New(Options{MaxEntries: 2})
	c.Set("a", 1)
	c.Set("b", 2)
	c.Get("a")    // a is now MRU, b is LRU
	c.Set("c", 3) // should evict b
	if _, ok := c.Get("b"); ok {
		t.Error("b should have been evicted as LRU")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("a should survive")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(Options{MaxEntries: 100, TTL: 500 * time.Millisecond})
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i%50)
			c.Set(key, i)
			c.Get(key)
			c.Delete(key)
		}(i)
	}
	wg.Wait()
}

func TestNoTTLNeverExpires(t *testing.T) {
	c := New()
	c.Set("k", "v")
	time.Sleep(10 * time.Millisecond)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("entry without TTL should not expire")
	}
}

func TestGCRemovesExpired(t *testing.T) {
	// TTL/3 gc interval means gc fires at ~17ms for a 50ms TTL
	c := New(Options{TTL: 50 * time.Millisecond})
	c.Set("k", "v")
	time.Sleep(120 * time.Millisecond) // wait for gc to run at least once post-expiry

	c.mu.Lock()
	_, inMap := c.items["k"]
	_, inExp := c.expiration["k"]
	c.mu.Unlock()

	if inMap || inExp {
		t.Fatal("gc should have removed expired entry from internal maps")
	}
}
