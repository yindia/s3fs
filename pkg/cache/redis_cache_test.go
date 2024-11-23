package cache

import (
	"context"
	"testing"

	"github.com/go-redis/redismock/v8"
)

func TestRedisCache(t *testing.T) {
	// Create a mock Redis client
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{
		client: db,
		ctx:    context.Background(),
	}

	// Test Set
	key := "testKey"
	value := []byte("testValue")
	mock.ExpectSet(key, value, 0).SetVal("OK")

	if err := cache.Set(key, value); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test Get
	mock.ExpectGet(key).SetVal(string(value))

	retrievedValue, exists := cache.Get(key)
	if !exists || string(retrievedValue) != string(value) {
		t.Fatalf("expected %s, got %s", value, retrievedValue)
	}

	// Test Delete
	mock.ExpectDel(key).SetVal(1)

	if err := cache.Delete(key); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestRedisCache_GetNonExistentKey(t *testing.T) {
	// Create a mock Redis client
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{
		client: db,
		ctx:    context.Background(),
	}

	key := "nonExistentKey"
	mock.ExpectGet(key).RedisNil()

	retrievedValue, exists := cache.Get(key)
	if exists {
		t.Fatalf("expected key %s to not exist", key)
	}
	if retrievedValue != nil {
		t.Fatalf("expected nil value, got %s", retrievedValue)
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}
