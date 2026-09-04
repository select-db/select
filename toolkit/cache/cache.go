package cache

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

type item struct {
	value   any
	element *list.Element // position in lru list; front = most recent
}

// A removal is an entry that has left the cache, carried out of the lock so
// OnRemove can run without the cache held.
type removal struct {
	key   string
	value any
}

// inflight is one GetOrCreate call in progress. Waiters block on done, then
// read value and err; closing done publishes those writes.
type inflight struct {
	done  chan struct{}
	value any
	err   error
}

// errCreatePanicked reaches waiters when the create function panicked. The panic
// itself unwinds through the caller that ran it.
var errCreatePanicked = errors.New("cache: create panicked")

// Options configures a Cache. Zero values mean no limit / no expiry.
type Options struct {
	MaxEntries int           // evicts LRU entry on overflow; 0 = unlimited
	TTL        time.Duration // entry lifetime; 0 = never expire

	// OnRemove is called once for every value that leaves the cache — expiry,
	// LRU eviction, Delete, DeleteFunc or replacement by Set. It runs with the
	// lock released, so it may block, and must not call back into this Cache.
	OnRemove func(key string, value any)
}

type Cache struct {
	mu sync.Mutex

	items      map[string]*item
	expiration map[string]int64     // key → expiry UnixNano; absent means no expiry
	lru        *list.List           // front = most recently used, back = least recently used
	creating   map[string]*inflight // keys with a GetOrCreate in progress

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
		creating:   make(map[string]*inflight),

		opts: o,
	}

	if o.TTL > 0 {
		go c.gc(o.TTL / 3)
	}

	return c
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	removed := c.setLocked(key, value, nil)
	c.mu.Unlock()
	c.notifyRemovals(removed)
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	value, ok, removed := c.getLocked(key, nil)
	c.mu.Unlock()
	c.notifyRemovals(removed)
	return value, ok
}

// GetOrCreate returns the value for key, calling create at most once across
// concurrent callers that miss: the others block and take the same result.
// A created value is stored; an error is not, so the next caller retries.
// create runs with the lock released and must not call back into this Cache.
func (c *Cache) GetOrCreate(key string, create func() (any, error)) (any, error) {
	c.mu.Lock()
	value, ok, removed := c.getLocked(key, nil)
	if ok {
		c.mu.Unlock()
		c.notifyRemovals(removed)
		return value, nil
	}
	if waitFor, busy := c.creating[key]; busy {
		c.mu.Unlock()
		c.notifyRemovals(removed)
		<-waitFor.done
		return waitFor.value, waitFor.err
	}
	fl := &inflight{done: make(chan struct{}), err: errCreatePanicked}
	c.creating[key] = fl
	c.mu.Unlock()
	c.notifyRemovals(removed)

	// Deferred so a panic in create cannot wedge the key: it is released, then
	// waiters wake on errCreatePanicked, which fl carries until create returns.
	defer close(fl.done)
	defer func() {
		c.mu.Lock()
		delete(c.creating, key)
		c.mu.Unlock()
	}()

	created, err := create()
	if err != nil {
		fl.value, fl.err = nil, err
		return nil, err
	}

	c.mu.Lock()
	stored := c.setLocked(key, created, nil)
	c.mu.Unlock()
	c.notifyRemovals(stored)

	fl.value, fl.err = created, nil
	return created, nil
}

func (c *Cache) Delete(key string) {
	var removed []removal
	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		removed = c.removeLocked(key, it, removed)
	}
	c.mu.Unlock()
	c.notifyRemovals(removed)
}

// DeleteFunc deletes all entries for which fn returns true.
func (c *Cache) DeleteFunc(fn func(key string) bool) {
	var removed []removal
	c.mu.Lock()
	for k, it := range c.items {
		if fn(k) {
			removed = c.removeLocked(k, it, removed)
		}
	}
	c.mu.Unlock()
	c.notifyRemovals(removed)
}

// getLocked returns the live value for key, refreshing its TTL and LRU position.
// An entry past its expiry is removed instead, and reported through removed.
// Caller must hold mu.
func (c *Cache) getLocked(key string, removed []removal) (any, bool, []removal) {
	it, ok := c.items[key]
	if !ok {
		return nil, false, removed
	}
	if exp, hasExp := c.expiration[key]; hasExp && time.Now().UnixNano() > exp {
		return nil, false, c.removeLocked(key, it, removed)
	}
	c.lru.MoveToFront(it.element)
	c.touchLocked(key)
	return it.value, true, removed
}

// setLocked stores value under key, appending whatever it displaces to removed:
// the previous value for this key, or the LRU victim that makes room for it.
// Caller must hold mu.
func (c *Cache) setLocked(key string, value any, removed []removal) []removal {
	if it, ok := c.items[key]; ok {
		if it.value != value && c.opts.OnRemove != nil {
			removed = append(removed, removal{key, it.value})
		}
		it.value = value
		c.lru.MoveToFront(it.element)
		c.touchLocked(key)
		return removed
	}

	if c.opts.MaxEntries > 0 && c.lru.Len() >= c.opts.MaxEntries {
		removed = c.evictLRULocked(removed)
	}

	c.items[key] = &item{value: value, element: c.lru.PushFront(key)}
	c.touchLocked(key)
	return removed
}

// evictLRULocked removes the least recently used entry — the only removal the
// cache decides on its own to stay under MaxEntries. Caller must hold mu.
func (c *Cache) evictLRULocked(removed []removal) []removal {
	el := c.lru.Back()
	if el == nil {
		return removed
	}
	key := el.Value.(string)
	if it, ok := c.items[key]; ok {
		return c.removeLocked(key, it, removed)
	}
	return removed
}

// removeLocked takes key out of the map, list and expiration index, appending it
// to removed for the caller to hand to OnRemove after unlocking. Every removal
// path goes through here. Caller must hold mu.
func (c *Cache) removeLocked(key string, it *item, removed []removal) []removal {
	c.lru.Remove(it.element)
	delete(c.items, key)
	delete(c.expiration, key)
	if c.opts.OnRemove != nil {
		removed = append(removed, removal{key, it.value})
	}
	return removed
}

// notifyRemovals runs OnRemove for entries taken out under the lock. Must be
// called with the lock released, since OnRemove may block.
func (c *Cache) notifyRemovals(removed []removal) {
	if c.opts.OnRemove == nil {
		return
	}
	for _, r := range removed {
		c.opts.OnRemove(r.key, r.value)
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
		var removed []removal
		c.mu.Lock()
		for k, exp := range c.expiration {
			if now > exp {
				if it, ok := c.items[k]; ok {
					removed = c.removeLocked(k, it, removed)
				}
			}
		}
		c.mu.Unlock()
		c.notifyRemovals(removed)
	}
}
