package cache

import "sync"

// MemoryCache is an in-memory implementation of the Cache interface
type MemoryCache struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewMemoryCache creates a new instance of MemoryCache
func NewMemoryCache() Cache {
	return &MemoryCache{
		data: make(map[string][]byte),
	}
}

// Set stores the value in the cache
func (m *MemoryCache) Set(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

// Get retrieves the value from the cache
func (m *MemoryCache) Get(key string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.data[key]
	return value, exists
}

// Delete removes the value from the cache
func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
