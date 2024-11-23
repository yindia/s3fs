package cache

import (
	"sync"
	"testing"
)

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()

	// Test Set and Get
	key := "testKey"
	value := []byte("testValue")
	if err := cache.Set(key, value); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrievedValue, exists := cache.Get(key)
	if !exists || string(retrievedValue) != string(value) {
		t.Fatalf("expected %s, got %s", value, retrievedValue)
	}

	// Test Delete
	if err := cache.Delete(key); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, exists = cache.Get(key)
	if exists {
		t.Fatalf("expected key %s to be deleted", key)
	}
}

// Additional tests

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache()
	key := "concurrentKey"
	value := []byte("concurrentValue")
	var wg sync.WaitGroup

	wg.Add(1)

	// Set value concurrently
	go func() {
		defer wg.Done()
		cache.Set(key, value)
	}()

	wg.Wait()

	// Get value concurrently
	retrievedValue, exists := cache.Get(key)
	if !exists || string(retrievedValue) != string(value) {
		t.Fatalf("expected %s, got %s", value, retrievedValue)
	}
}

func TestMemoryCache_OverwriteValue(t *testing.T) {
	cache := NewMemoryCache()
	key := "overwriteKey"
	value1 := []byte("value1")
	value2 := []byte("value2")

	// Set initial value
	if err := cache.Set(key, value1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Overwrite with new value
	if err := cache.Set(key, value2); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the new value
	retrievedValue, exists := cache.Get(key)
	if !exists || string(retrievedValue) != string(value2) {
		t.Fatalf("expected %s, got %s", value2, retrievedValue)
	}
}

func TestMemoryCache_DeleteNonExistentKey(t *testing.T) {
	cache := NewMemoryCache()
	key := "nonExistentKey"

	// Attempt to delete a non-existent key
	if err := cache.Delete(key); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the key still does not exist
	_, exists := cache.Get(key)
	if exists {
		t.Fatalf("expected key %s to not exist", key)
	}
}
