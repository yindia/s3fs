package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// RedisCache is a Redis implementation of the Cache interface
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisCache creates a new instance of RedisCache
func NewRedisCache(addr string) Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisCache{
		client: rdb,
		ctx:    context.Background(),
	}
}

// Set stores the value in the Redis cache
func (r *RedisCache) Set(key string, value []byte) error {
	return r.client.Set(r.ctx, key, value, 0).Err()
}

// Get retrieves the value from the Redis cache
func (r *RedisCache) Get(key string) ([]byte, bool) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false
		}
		return nil, false // Handle other errors as needed
	}
	return []byte(val), true
}

// Delete removes the value from the Redis cache
func (r *RedisCache) Delete(key string) error {
	return r.client.Del(r.ctx, key).Err()
}
