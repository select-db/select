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

// A deletion is an entry that has left the cache, carried out of the lock so
// OnDelete can run without the cache held.
type deletion struct {
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
	deleted := c.setLocked(key, value, nil)
	c.mu.Unlock()
	c.notifyDeletions(deleted)
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	value, ok, deleted := c.getLocked(key, nil)
	c.mu.Unlock()
	c.notifyDeletions(deleted)
	return value, ok
}

// GetOrCreate returns the value for key, calling create at most once across
// concurrent callers that miss: the others block and take the same result.
// A created value is stored; an error is not, so the next caller retries.
// create runs with the lock released and must not call back into this Cache.
func (c *Cache) GetOrCreate(key string, create func() (any, error)) (any, error) {
	c.mu.Lock()
	value, ok, deleted := c.getLocked(key, nil)
	if ok {
		c.mu.Unlock()
		c.notifyDeletions(deleted)
		return value, nil
	}
	if waitFor, busy := c.creating[key]; busy {
		c.mu.Unlock()
		c.notifyDeletions(deleted)
		<-waitFor.done
		return waitFor.value, waitFor.err
	}
	fl := &inflight{done: make(chan struct{}), err: errCreatePanicked}
	c.creating[key] = fl
	c.mu.Unlock()
	c.notifyDeletions(deleted)

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
	c.notifyDeletions(stored)

	fl.value, fl.err = created, nil
	return created, nil
}

func (c *Cache) Delete(key string) {
	var deleted []deletion
	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		deleted = c.deleteLocked(key, it, deleted)
	}
	c.mu.Unlock()
	c.notifyDeletions(deleted)
}

// DeleteFunc deletes all entries for which fn returns true.
func (c *Cache) DeleteFunc(fn func(key string) bool) {
	var deleted []deletion
	c.mu.Lock()
	for k, it := range c.items {
		if fn(k) {
			deleted = c.deleteLocked(k, it, deleted)
		}
	}
	c.mu.Unlock()
	c.notifyDeletions(deleted)
}

// getLocked returns the live value for key, refreshing its TTL and LRU position.
// An entry past its expiry is deleted instead, and reported through deleted.
// Caller must hold mu.
func (c *Cache) getLocked(key string, deleted []deletion) (any, bool, []deletion) {
	it, ok := c.items[key]
	if !ok {
		return nil, false, deleted
	}
	if exp, hasExp := c.expiration[key]; hasExp && time.Now().UnixNano() > exp {
		return nil, false, c.deleteLocked(key, it, deleted)
	}
	c.lru.MoveToFront(it.element)
	c.touchLocked(key)
	return it.value, true, deleted
}

// setLocked stores value under key, appending whatever it displaces to deleted:
// the previous value for this key, or the LRU victim that makes room for it.
// Caller must hold mu.
func (c *Cache) setLocked(key string, value any, deleted []deletion) []deletion {
	if it, ok := c.items[key]; ok {
		if it.value != value && c.opts.OnDelete != nil {
			deleted = append(deleted, deletion{key, it.value})
		}
		it.value = value
		c.lru.MoveToFront(it.element)
		c.touchLocked(key)
		return deleted
	}

	if c.opts.MaxEntries > 0 && c.lru.Len() >= c.opts.MaxEntries {
		deleted = c.deleteLRULocked(deleted)
	}

	c.items[key] = &item{value: value, element: c.lru.PushFront(key)}
	c.touchLocked(key)
	return deleted
}

// deleteLRULocked deletes the least recently used entry — the only deletion the
// cache decides on its own to stay under MaxEntries. Caller must hold mu.
func (c *Cache) deleteLRULocked(deleted []deletion) []deletion {
	el := c.lru.Back()
	if el == nil {
		return deleted
	}
	key := el.Value.(string)
	if it, ok := c.items[key]; ok {
		return c.deleteLocked(key, it, deleted)
	}
	return deleted
}

// deleteLocked takes key out of the map, list and expiration index, appending it
// to deleted for the caller to hand to OnDelete after unlocking. Every deletion
// path goes through here. Caller must hold mu.
func (c *Cache) deleteLocked(key string, it *item, deleted []deletion) []deletion {
	c.lru.Remove(it.element)
	delete(c.items, key)
	delete(c.expiration, key)
	if c.opts.OnDelete != nil {
		deleted = append(deleted, deletion{key, it.value})
	}
	return deleted
}

// notifyDeletions runs OnDelete for entries taken out under the lock. Must be
// called with the lock released, since OnDelete may block.
func (c *Cache) notifyDeletions(deleted []deletion) {
	if c.opts.OnDelete == nil {
		return
	}
	for _, r := range deleted {
		c.opts.OnDelete(r.key, r.value)
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
		var deleted []deletion
		c.mu.Lock()
		for k, exp := range c.expiration {
			if now > exp {
				if it, ok := c.items[k]; ok {
					deleted = c.deleteLocked(k, it, deleted)
				}
			}
		}
		c.mu.Unlock()
		c.notifyDeletions(deleted)
	}
}
