package cache

import (
	"s3fs/pkg/config"
)

// Cache interface defines the methods for caching
type Cache interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, bool)
	GetAll() ([][]byte, bool)
	Delete(key string) error
}

// NewRedisCache creates a new instance of RedisCache
func NewCache(cacheType string, conf *config.Config) Cache {

	if cacheType == "redis" {
		return NewRedisCache(conf)
	}
	return NewMemoryCache()
}
