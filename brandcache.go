package katsuragi

import (
	"container/list"
	"sync"
	"time"
)

// BrandCache caches brand(+hint) -> resolved-domain lookups made by
// GetFaviconByBrand. Implement this against your own store (Redis,
// Memcached, a database, ...) to share brand resolutions across a
// horizontally-scaled deployment; the default (InMemoryBrandCache) is
// process-local and won't be shared across instances.
type BrandCache interface {
	// Get returns the cached domain for key, if present and not expired.
	Get(key string) (domain string, ok bool)
	// Set stores domain under key, replacing any existing entry.
	Set(key, domain string)
}

type brandCacheEntry struct {
	key       string
	domain    string
	expiresAt time.Time
}

// InMemoryBrandCache is the default BrandCache: a process-local LRU with a
// per-entry TTL, since brand->domain mappings are far more stable than page
// HTML but can still go stale (rebrands, acquisitions). Not suitable for
// horizontally-scaled deployments - see BrandCache.
//
// The zero value is ready to use.
type InMemoryBrandCache struct {
	// Capacity is the maximum number of entries retained; the
	// least-recently-used entry is evicted once exceeded. Defaults to
	// 1000 when zero.
	Capacity int
	// TTL is how long an entry remains valid after being set. Defaults
	// to 24h when zero.
	TTL time.Duration

	once  sync.Once
	mu    sync.Mutex
	items map[string]*list.Element
	lru   *list.List
}

const defaultBrandCacheCapacity = 1000
const defaultBrandCacheTTL = 24 * time.Hour

func (c *InMemoryBrandCache) init() {
	c.once.Do(func() {
		if c.Capacity == 0 {
			c.Capacity = defaultBrandCacheCapacity
		}
		if c.TTL == 0 {
			c.TTL = defaultBrandCacheTTL
		}
		c.items = make(map[string]*list.Element)
		c.lru = list.New()
	})
}

var _ BrandCache = (*InMemoryBrandCache)(nil)

// Get implements BrandCache. Like Fetcher.GetFromCache (cache.go), this
// takes the exclusive lock rather than a read lock, because a hit calls
// lru.MoveToFront, which mutates the shared LRU list.
func (c *InMemoryBrandCache) Get(key string) (string, bool) {
	c.init()
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return "", false
	}
	entry := elem.Value.(*brandCacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.lru.Remove(elem)
		delete(c.items, key)
		return "", false
	}
	c.lru.MoveToFront(elem)
	return entry.domain, true
}

// Set implements BrandCache.
func (c *InMemoryBrandCache) Set(key, domain string) {
	c.init()
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(c.TTL)
	if elem, ok := c.items[key]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*brandCacheEntry)
		entry.domain = domain
		entry.expiresAt = expiresAt
		return
	}

	if len(c.items) >= c.Capacity {
		oldest := c.lru.Back()
		if oldest != nil {
			delete(c.items, oldest.Value.(*brandCacheEntry).key)
			c.lru.Remove(oldest)
		}
	}

	entry := &brandCacheEntry{key: key, domain: domain, expiresAt: expiresAt}
	elem := c.lru.PushFront(entry)
	c.items[key] = elem
}
