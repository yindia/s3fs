package cache

import (
	"testing"

	"github.com/go-redis/redis/v8"
)

// TestNewCache tests the NewCache function
func TestNewCache(t *testing.T) {
	conf := &redis.Options{
		Addr: "localhost:6379", // Example Redis address
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
