package explorer

import (
	"container/list"
	"sync"
)

// lruCache is a small, thread-safe LRU used for prevout and block-header
// lookups. Values are immutable once cached (chain data), so entries never
// need invalidation — only bounded eviction.
type lruCache[V any] struct {
	mu    sync.Mutex
	cap   int
	order *list.List // front = most recently used; values are *lruEntry[V]
	items map[string]*list.Element
}

type lruEntry[V any] struct {
	key string
	val V
}

func newLRU[V any](capacity int) *lruCache[V] {
	return &lruCache[V]{
		cap:   capacity,
		order: list.New(),
		items: make(map[string]*list.Element, capacity),
	}
}

func (c *lruCache[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruEntry[V]).val, true
}

func (c *lruCache[V]) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	c.items = make(map[string]*list.Element, c.cap)
}

func (c *lruCache[V]) put(key string, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		el.Value.(*lruEntry[V]).val = val
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry[V]{key: key, val: val})
	if c.order.Len() > c.cap {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*lruEntry[V]).key)
	}
}
