package cache

import (
	"s3fs/pkg/config"
	"testing"
)

// TestNewCache tests the NewCache function
func TestNewCache(t *testing.T) {
	conf := &config.Config{
		Cache: config.CacheConfig{
			Host: "localhost",
			Port: 6379,
		},
	}

	// Test for Redis cache
	cache := NewCache("redis", conf)
	if _, ok := cache.(*RedisCache); !ok {
		t.Errorf("Expected *RedisCache, got %T", cache)
	}

	// Test for Memory cache
	cache = NewCache("memory", nil)
	if _, ok := cache.(*MemoryCache); !ok {
		t.Errorf("Expected MemoryCache, got %T", cache)
	}
}
