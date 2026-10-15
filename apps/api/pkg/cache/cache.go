package cache

import (
	"sync"
	"time"
)

// Item represents a cache item
type Item struct {
	Value      interface{}
	Expiration int64 // Unix timestamp in nanoseconds
}

// Cache is a simple in-memory cache with TTL
type Cache struct {
	items map[string]Item
	mu    sync.RWMutex
	ttl   time.Duration
}

// New creates a new cache with the given default TTL
func New(ttl time.Duration) *Cache {
	c := &Cache{
		items: make(map[string]Item),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

// Get returns a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	if time.Now().UnixNano() > item.Expiration {
		return nil, false
	}

	return item.Value, true
}

// Set adds a value to the cache with the default TTL
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL adds a value to the cache with a specific TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item{
		Value:      value,
		Expiration: time.Now().Add(ttl).UnixNano(),
	}
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]Item)
}

// cleanup periodically removes expired items
func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now().UnixNano()
		for k, v := range c.items {
			if now > v.Expiration {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
