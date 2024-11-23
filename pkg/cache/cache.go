package cache

import (
	"github.com/go-redis/redis/v8"
)

// Cache interface defines the methods for caching
type Cache interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, bool)
	Delete(key string) error
}

// NewRedisCache creates a new instance of RedisCache
func NewCache(cacheType string, conf *redis.Options) Cache {

	if cacheType == "redis" {
		return NewRedisCache(conf.Addr)
	}
	return NewMemoryCache()
}
