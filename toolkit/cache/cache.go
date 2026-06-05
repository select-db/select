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

// Options configures a Cache. Zero values mean no limit / no expiry.
type Options struct {
	MaxEntries int           // evicts LRU entry on overflow; 0 = unlimited
	TTL        time.Duration // entry lifetime; 0 = never expire
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if it, ok := c.items[key]; ok {
		it.value = value
		c.lru.MoveToFront(it.element)
		c.touchLocked(key)
		return
	}

	if c.opts.MaxEntries > 0 && c.lru.Len() >= c.opts.MaxEntries {
		c.evictLocked()
	}

	el := c.lru.PushFront(key)
	c.items[key] = &item{value: value, element: el}
	c.touchLocked(key)
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	it, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if exp, hasExp := c.expiration[key]; hasExp && time.Now().UnixNano() > exp {
		c.removeLocked(key, it)
		return nil, false
	}

	c.lru.MoveToFront(it.element)
	c.touchLocked(key)
	return it.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		c.removeLocked(key, it)
	}
	c.mu.Unlock()
}

// DeleteFunc deletes all entries for which fn returns true.
func (c *Cache) DeleteFunc(fn func(key string) bool) {
	c.mu.Lock()
	for k, it := range c.items {
		if fn(k) {
			c.removeLocked(k, it)
		}
	}
	c.mu.Unlock()
}

// evictLocked removes the least recently used entry. Caller must hold mu.
func (c *Cache) evictLocked() {
	el := c.lru.Back()
	if el == nil {
		return
	}
	key := el.Value.(string)
	if it, ok := c.items[key]; ok {
		c.removeLocked(key, it)
	}
}

// removeLocked removes key from map, list, and expiration index. Caller must hold mu.
func (c *Cache) removeLocked(key string, it *item) {
	c.lru.Remove(it.element)
	delete(c.items, key)
	delete(c.expiration, key)
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
		c.mu.Lock()
		for k, exp := range c.expiration {
			if now > exp {
				if it, ok := c.items[k]; ok {
					c.removeLocked(k, it)
				}
			}
		}
		c.mu.Unlock()
	}
}
