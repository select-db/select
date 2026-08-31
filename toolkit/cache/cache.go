package cache

import (
	"container/list"
	"sync"
	"time"
)

type item struct {
	value   any
	element *list.Element // position in lru list; front = most recent
}

// evictedEntry carries a removed entry out of the lock so OnEvict can run
// without the cache held.
type evictedEntry struct {
	key   string
	value any
}

// Options configures a Cache. Zero values mean no limit / no expiry.
type Options struct {
	MaxEntries int           // evicts LRU entry on overflow; 0 = unlimited
	TTL        time.Duration // entry lifetime; 0 = never expire

	// OnEvict, when set, is called once for every value that leaves the cache:
	// TTL expiry, LRU overflow, Delete, DeleteFunc, or replacement by Set.
	// Use it for values that own resources the GC cannot reclaim on its own,
	// such as a *sql.DB whose pool must be closed.
	//
	// It runs after the cache lock is released, so it may block, and it must
	// not call back into the same Cache.
	OnEvict func(key string, value any)
}

type Cache struct {
	mu sync.Mutex

	items      map[string]*item
	expiration map[string]int64 // key → expiry UnixNano; absent means no expiry
	lru        *list.List       // front = most recently used, back = least recently used

	opts Options
}

func New(opts ...Options) *Cache {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}

	c := &Cache{
		items:      make(map[string]*item),
		expiration: make(map[string]int64),
		lru:        list.New(),

		opts: o,
	}

	if o.TTL > 0 {
		go c.gc(o.TTL / 3)
	}

	return c
}

func (c *Cache) Set(key string, value any) {
	var evicted []evictedEntry

	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		if it.value != value {
			evicted = append(evicted, evictedEntry{key, it.value})
		}
		it.value = value
		c.lru.MoveToFront(it.element)
		c.touchLocked(key)
		c.mu.Unlock()
		c.fireEvictions(evicted)
		return
	}

	if c.opts.MaxEntries > 0 && c.lru.Len() >= c.opts.MaxEntries {
		evicted = c.evictLocked(evicted)
	}

	el := c.lru.PushFront(key)
	c.items[key] = &item{value: value, element: el}
	c.touchLocked(key)
	c.mu.Unlock()

	c.fireEvictions(evicted)
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()

	it, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		return nil, false
	}

	if exp, hasExp := c.expiration[key]; hasExp && time.Now().UnixNano() > exp {
		expired := c.removeLocked(key, it, nil)
		c.mu.Unlock()
		c.fireEvictions(expired)
		return nil, false
	}

	c.lru.MoveToFront(it.element)
	c.touchLocked(key)
	value := it.value
	c.mu.Unlock()
	return value, true
}

func (c *Cache) Delete(key string) {
	var evicted []evictedEntry
	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		evicted = c.removeLocked(key, it, evicted)
	}
	c.mu.Unlock()
	c.fireEvictions(evicted)
}

// DeleteFunc deletes all entries for which fn returns true.
func (c *Cache) DeleteFunc(fn func(key string) bool) {
	var evicted []evictedEntry
	c.mu.Lock()
	for k, it := range c.items {
		if fn(k) {
			evicted = c.removeLocked(k, it, evicted)
		}
	}
	c.mu.Unlock()
	c.fireEvictions(evicted)
}

// evictLocked removes the least recently used entry, appending it to evicted.
// Caller must hold mu.
func (c *Cache) evictLocked(evicted []evictedEntry) []evictedEntry {
	el := c.lru.Back()
	if el == nil {
		return evicted
	}
	key := el.Value.(string)
	if it, ok := c.items[key]; ok {
		return c.removeLocked(key, it, evicted)
	}
	return evicted
}

// removeLocked removes key from map, list, and expiration index, appending the
// removed value to evicted for the caller to hand to OnEvict after unlocking.
// Caller must hold mu.
func (c *Cache) removeLocked(key string, it *item, evicted []evictedEntry) []evictedEntry {
	c.lru.Remove(it.element)
	delete(c.items, key)
	delete(c.expiration, key)
	if c.opts.OnEvict != nil {
		evicted = append(evicted, evictedEntry{key, it.value})
	}
	return evicted
}

// fireEvictions runs OnEvict for entries removed under the lock. Must be called
// with the lock released: OnEvict may block (closing a connection pool, say).
func (c *Cache) fireEvictions(evicted []evictedEntry) {
	if c.opts.OnEvict == nil {
		return
	}
	for _, e := range evicted {
		c.opts.OnEvict(e.key, e.value)
	}
}

// touchLocked reset expiration for key. Caller must hold mu.
func (c *Cache) touchLocked(key string) {
	if c.opts.TTL == 0 {
		return
	}
	c.expiration[key] = time.Now().Add(c.opts.TTL).UnixNano()
}

func (c *Cache) gc(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now().UnixNano()
		var evicted []evictedEntry
		c.mu.Lock()
		for k, exp := range c.expiration {
			if now > exp {
				if it, ok := c.items[k]; ok {
					evicted = c.removeLocked(k, it, evicted)
				}
			}
		}
		c.mu.Unlock()
		c.fireEvictions(evicted)
	}
}
