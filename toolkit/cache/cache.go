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

// A deletedItem is an item that has left the cache, held by key and value until
// the lock is released so OnDelete can run without it.
type deletedItem struct {
	key   string
	value any
}

// A call is one GetOrCreate in progress for a key. Waiters block on done, then
// read value and err; closing done publishes those writes. Named as in
// golang.org/x/sync/singleflight, which suppresses duplicates the same way.
type call struct {
	done  chan struct{}
	value any
	err   error
}

// Options configures a Cache. Zero values mean no limit / no expiry.
type Options struct {
	MaxEntries int           // deletes LRU entry on overflow; 0 = unlimited
	TTL        time.Duration // entry lifetime; 0 = never expire

	// OnDelete is called once for every value that leaves the cache — expiry,
	// LRU overflow, Delete, DeleteFunc or replacement by Set. It runs with the
	// lock released, so it may block, and must not call back into this Cache.
	OnDelete func(key string, value any)
}

type Cache struct {
	mu sync.Mutex

	items      map[string]*item
	expiration map[string]int64 // key → expiry UnixNano; absent means no expiry
	lru        *list.List       // front = most recently used, back = least recently used
	creating   map[string]*call // keys with a GetOrCreate in progress

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
		creating:   make(map[string]*call),

		opts: o,
	}

	if o.TTL > 0 {
		go c.gc(o.TTL / 3)
	}

	return c
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()

	it, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		return nil, false
	}
	if c.expiredLocked(key) {
		value := c.deleteLocked(key, it)
		c.mu.Unlock()
		c.notifyDelete(key, value)
		return nil, false
	}

	c.markUsedLocked(key, it)
	value := it.value
	c.mu.Unlock()
	return value, true
}

// GetOrCreate returns the value for key, calling create at most once across
// concurrent callers that miss: the others block and take the same result.
// A created value is stored; an error is not, so the next caller retries.
// create runs with the lock released and must not call back into this Cache.
func (c *Cache) GetOrCreate(key string, create func() (any, error)) (any, error) {
	c.mu.Lock()

	var expired deletedItem
	var wasExpired bool
	if it, ok := c.items[key]; ok {
		if !c.expiredLocked(key) {
			c.markUsedLocked(key, it)
			value := it.value
			c.mu.Unlock()
			return value, nil
		}
		expired, wasExpired = deletedItem{key, c.deleteLocked(key, it)}, true
	}

	if waitFor, busy := c.creating[key]; busy {
		c.mu.Unlock()
		if wasExpired {
			c.notifyDelete(expired.key, expired.value)
		}
		<-waitFor.done
		return waitFor.value, waitFor.err
	}

	pending := &call{
		done: make(chan struct{}),
		// Held until create returns, so if create panics the waiters get an
		// error rather than a nil result. The panic unwinds through the caller
		// that ran it.
		err: errors.New("cache: create panicked"),
	}
	c.creating[key] = pending
	c.mu.Unlock()
	if wasExpired {
		c.notifyDelete(expired.key, expired.value)
	}

	// Deferred so a panic in create cannot wedge the key: it is released, then
	// the waiters wake on the error pending was built with.
	defer close(pending.done)
	defer func() {
		c.mu.Lock()
		delete(c.creating, key)
		c.mu.Unlock()
	}()

	created, err := create()
	if err != nil {
		pending.value, pending.err = nil, err
		return nil, err
	}

	c.Set(key, created)
	pending.value, pending.err = created, nil
	return created, nil
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()

	if it, ok := c.items[key]; ok {
		replaced := c.replaceLocked(key, it, value)
		c.mu.Unlock()
		if replaced != value {
			c.notifyDelete(key, replaced)
		}
		return
	}

	var victim deletedItem
	var haveVictim bool
	if c.fullLocked() {
		if victimKey, victimItem, ok := c.lruVictimLocked(); ok {
			victim, haveVictim = deletedItem{victimKey, c.deleteLocked(victimKey, victimItem)}, true
		}
	}
	c.insertLocked(key, value)
	c.mu.Unlock()

	if haveVictim {
		c.notifyDelete(victim.key, victim.value)
	}
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	it, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	value := c.deleteLocked(key, it)
	c.mu.Unlock()
	c.notifyDelete(key, value)
}

// DeleteFunc deletes all entries for which fn returns true.
func (c *Cache) DeleteFunc(fn func(key string) bool) {
	c.mu.Lock()
	var deleted []deletedItem
	for key, it := range c.items {
		if fn(key) {
			deleted = append(deleted, deletedItem{key, c.deleteLocked(key, it)})
		}
	}
	c.mu.Unlock()

	for _, d := range deleted {
		c.notifyDelete(d.key, d.value)
	}
}

// expiredLocked reports whether key is past its expiry. Caller must hold mu.
func (c *Cache) expiredLocked(key string) bool {
	exp, ok := c.expiration[key]
	return ok && time.Now().UnixNano() > exp
}

// markUsedLocked moves key to the front of the LRU list and restarts its TTL.
// Caller must hold mu.
func (c *Cache) markUsedLocked(key string, it *item) {
	c.lru.MoveToFront(it.element)
	if c.opts.TTL > 0 {
		c.expiration[key] = time.Now().Add(c.opts.TTL).UnixNano()
	}
}

// insertLocked adds a new entry at the front of the LRU list. The caller must
// have established that key is absent and that there is room. Caller must hold mu.
func (c *Cache) insertLocked(key string, value any) {
	c.items[key] = &item{value: value, element: c.lru.PushFront(key)}
	if c.opts.TTL > 0 {
		c.expiration[key] = time.Now().Add(c.opts.TTL).UnixNano()
	}
}

// replaceLocked swaps in a new value for an entry that is already present,
// returning the value it displaced. Caller must hold mu.
func (c *Cache) replaceLocked(key string, it *item, value any) any {
	replaced := it.value
	it.value = value
	c.markUsedLocked(key, it)
	return replaced
}

// deleteLocked takes key out of the items map, the expiry index and the LRU
// list, returning the value it held. Every deletion goes through here, and none
// of them calls OnDelete: that is the caller's job, once it has released the
// lock. Caller must hold mu.
func (c *Cache) deleteLocked(key string, it *item) any {
	c.lru.Remove(it.element)
	delete(c.items, key)
	delete(c.expiration, key)
	return it.value
}

// fullLocked reports whether the cache is at MaxEntries. Caller must hold mu.
func (c *Cache) fullLocked() bool {
	return c.opts.MaxEntries > 0 && c.lru.Len() >= c.opts.MaxEntries
}

// lruVictimLocked returns the least recently used entry, the one an insert over
// MaxEntries deletes to make room. Caller must hold mu.
func (c *Cache) lruVictimLocked() (string, *item, bool) {
	el := c.lru.Back()
	if el == nil {
		return "", nil, false
	}
	key := el.Value.(string)
	it, ok := c.items[key]
	return key, it, ok
}

// notifyDelete hands one deleted entry to OnDelete. Must be called with the
// lock released, since OnDelete may block.
func (c *Cache) notifyDelete(key string, value any) {
	if c.opts.OnDelete == nil {
		return
	}
	c.opts.OnDelete(key, value)
}

func (c *Cache) gc(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		c.mu.Lock()
		var deleted []deletedItem
		for key := range c.expiration {
			if c.expiredLocked(key) {
				if it, ok := c.items[key]; ok {
					deleted = append(deleted, deletedItem{key, c.deleteLocked(key, it)})
				}
			}
		}
		c.mu.Unlock()

		for _, d := range deleted {
			c.notifyDelete(d.key, d.value)
		}
	}
}
