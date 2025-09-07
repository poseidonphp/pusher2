package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// BASIC FUNCTIONALITY TESTS
// ============================================================================

func TestLocalCacheInit(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := cache.Init(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, cache.cacheItems)
}

func TestLocalCacheSetAndGet(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Test basic set and get
	cache.Set("key1", "value1")
	val, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	// Test getting non-existent key
	_, exists = cache.Get("nonexistent")
	assert.False(t, exists)
}

func TestLocalCacheSetExWithExpiration(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Set with short expiration
	cache.SetEx("key1", "value1", 500*time.Millisecond)

	// Should exist immediately
	val, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	// Wait for expiration
	time.Sleep(1000 * time.Millisecond)

	// Should be gone after expiration
	_, exists = cache.Get("key1")
	assert.False(t, exists)
}

func TestLocalCacheDelete(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Set and then delete
	cache.Set("key1", "value1")
	cache.Delete("key1")

	_, exists := cache.Get("key1")
	assert.False(t, exists)

	// Delete non-existent key should not cause errors
	cache.Delete("nonexistent")
}

func TestLocalCacheHas(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Test with non-existent key
	assert.False(t, cache.Has("key1"))

	// Test with existing key
	cache.Set("key1", "value1")
	assert.True(t, cache.Has("key1"))

	// Test with expired key
	cache.SetEx("key2", "value2", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	assert.False(t, cache.Has("key2"))
}

func TestLocalCacheUpdate(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Update non-existent key should do nothing
	cache.Update("key1", "new-value")
	assert.False(t, cache.Has("key1"))

	// Update existing key should change the value
	cache.Set("key1", "value1")
	cache.Update("key1", "new-value")

	val, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "new-value", val)
}

func TestLocalCacheRemember(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Test with callback that returns a value
	callCount := 0
	callback := func() (string, error) {
		callCount++
		return "computed-value", nil
	}

	// First call should execute callback
	val, err := cache.Remember("key1", 60, callback)
	assert.NoError(t, err)
	assert.Equal(t, "computed-value", val)
	assert.Equal(t, 1, callCount)

	// Second call should use cached value
	val, err = cache.Remember("key1", 60, callback)
	assert.NoError(t, err)
	assert.Equal(t, "computed-value", val)
	assert.Equal(t, 1, callCount) // Should still be 1

	// Test with callback that returns an error
	errorCallback := func() (string, error) {
		return "", assert.AnError
	}

	_, err = cache.Remember("key2", 60, errorCallback)
	assert.Error(t, err)
}

// ============================================================================
// CLEANUP JOB TESTS
// ============================================================================

func TestCleanupJob_Function(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Add items with different expiration times
	now := time.Now()

	// Item that expires in 100ms
	expireTime1 := now.Add(100 * time.Millisecond)
	cache.mu.Lock()
	cache.cacheItems["expired1"] = &localCacheItem{
		data:     "value1",
		expireAt: &expireTime1,
	}
	cache.mu.Unlock()

	// Item that expires in 200ms
	expireTime2 := now.Add(200 * time.Millisecond)
	cache.mu.Lock()
	cache.cacheItems["expired2"] = &localCacheItem{
		data:     "value2",
		expireAt: &expireTime2,
	}
	cache.mu.Unlock()

	// Item that never expires (nil expireAt)
	cache.mu.Lock()
	cache.cacheItems["permanent"] = &localCacheItem{
		data:     "value3",
		expireAt: nil,
	}
	cache.mu.Unlock()

	// Item that expires far in the future
	futureExpireTime := now.Add(1 * time.Hour)
	cache.mu.Lock()
	cache.cacheItems["future"] = &localCacheItem{
		data:     "value4",
		expireAt: &futureExpireTime,
	}
	cache.mu.Unlock()

	// Verify all items exist initially
	assert.True(t, cache.Has("expired1"))
	assert.True(t, cache.Has("expired2"))
	assert.True(t, cache.Has("permanent"))
	assert.True(t, cache.Has("future"))

	// Wait for items to expire
	time.Sleep(300 * time.Millisecond)

	// Simulate the cleanupJob function logic
	cache.mu.Lock()
	now = time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// Verify expired items are removed
	assert.False(t, cache.Has("expired1"))
	assert.False(t, cache.Has("expired2"))

	// Verify non-expired items remain
	assert.True(t, cache.Has("permanent"))
	assert.True(t, cache.Has("future"))
}

func TestCleanupJob_EmptyCache(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Simulate cleanupJob on empty cache
	cache.mu.Lock()
	now := time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// Should not cause any issues
	assert.Equal(t, 0, len(cache.cacheItems))
}

func TestCleanupJob_OnlyPermanentItems(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Add only permanent items (nil expireAt)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Simulate cleanupJob
	cache.mu.Lock()
	now := time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// All items should still exist
	assert.True(t, cache.Has("key1"))
	assert.True(t, cache.Has("key2"))
}

func TestCleanupJob_AllExpiredItems(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	now := time.Now()

	// Add only expired items
	pastExpireTime1 := now.Add(-1 * time.Hour)
	cache.mu.Lock()
	cache.cacheItems["expired1"] = &localCacheItem{
		data:     "value1",
		expireAt: &pastExpireTime1,
	}
	cache.mu.Unlock()

	pastExpireTime2 := now.Add(-2 * time.Hour)
	cache.mu.Lock()
	cache.cacheItems["expired2"] = &localCacheItem{
		data:     "value2",
		expireAt: &pastExpireTime2,
	}
	cache.mu.Unlock()

	// Simulate cleanupJob
	cache.mu.Lock()
	now = time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// All items should be removed
	assert.False(t, cache.Has("expired1"))
	assert.False(t, cache.Has("expired2"))
	assert.Equal(t, 0, len(cache.cacheItems))
}

func TestCleanupJob_ConcurrentModification(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Add some items
	cache.Set("key1", "value1")
	cache.SetEx("key2", "value2", 100*time.Millisecond)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Simulate concurrent access during cleanup
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Add new items
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			cache.Set("concurrent_key_"+string(rune(i)), "value")
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Simulate cleanupJob
	go func() {
		defer wg.Done()
		cache.mu.Lock()
		now := time.Now()
		for key, item := range cache.cacheItems {
			if item.expireAt != nil && now.After(*item.expireAt) {
				delete(cache.cacheItems, key)
			}
		}
		cache.mu.Unlock()
	}()

	wg.Wait()

	// Verify expired item is gone but new items remain
	assert.False(t, cache.Has("key2"))
	assert.True(t, cache.Has("key1"))
}

func TestCleanupJob_EdgeCases(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	now := time.Now()

	// Test item that expired in the past
	pastTime := now.Add(-1 * time.Hour)
	cache.mu.Lock()
	cache.cacheItems["expired"] = &localCacheItem{
		data:     "value1",
		expireAt: &pastTime,
	}
	cache.mu.Unlock()

	// Test item that expires in the future
	futureTime := now.Add(1 * time.Hour)
	cache.mu.Lock()
	cache.cacheItems["future"] = &localCacheItem{
		data:     "value2",
		expireAt: &futureTime,
	}
	cache.mu.Unlock()

	// Test item with nil expireAt (permanent)
	cache.mu.Lock()
	cache.cacheItems["permanent"] = &localCacheItem{
		data:     "value3",
		expireAt: nil,
	}
	cache.mu.Unlock()

	// Simulate cleanupJob
	cache.mu.Lock()
	cleanupTime := time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && cleanupTime.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// Items that expired before cleanup time should be removed
	assert.False(t, cache.Has("expired"))

	// Items that expire after cleanup time or are permanent should remain
	assert.True(t, cache.Has("future"))
	assert.True(t, cache.Has("permanent"))
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestLocalCacheExpirationIntegration(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Set with infinite TTL (nil expireAt)
	cache.Set("key1", "value1")

	// Should still exist after some time
	time.Sleep(100 * time.Millisecond)
	_, exists := cache.Get("key1")
	assert.True(t, exists)

	// Test that expired items are deleted when accessed
	expTime := time.Now().Add(100 * time.Millisecond)
	cache.mu.Lock()
	cache.cacheItems["key2"] = &localCacheItem{
		data:     "value2",
		expireAt: &expTime,
	}
	cache.mu.Unlock()

	time.Sleep(200 * time.Millisecond)
	_, exists = cache.Get("key2")
	assert.False(t, exists)
}

func TestLocalCacheMultipleOperations(t *testing.T) {
	cache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = cache.Init(ctx)

	// Set multiple items
	cache.Set("permanent1", "value1")
	cache.SetEx("temporary1", "value2", 100*time.Millisecond)
	cache.Set("permanent2", "value3")
	cache.SetEx("temporary2", "value4", 200*time.Millisecond)

	// Verify all exist
	assert.True(t, cache.Has("permanent1"))
	assert.True(t, cache.Has("temporary1"))
	assert.True(t, cache.Has("permanent2"))
	assert.True(t, cache.Has("temporary2"))

	// Wait for first temporary to expire
	time.Sleep(150 * time.Millisecond)

	// Manually trigger cleanup
	cache.mu.Lock()
	now := time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// First temporary should be gone, others should remain
	assert.True(t, cache.Has("permanent1"))
	assert.False(t, cache.Has("temporary1"))
	assert.True(t, cache.Has("permanent2"))
	assert.True(t, cache.Has("temporary2"))

	// Wait for second temporary to expire
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup again
	cache.mu.Lock()
	now = time.Now()
	for key, item := range cache.cacheItems {
		if item.expireAt != nil && now.After(*item.expireAt) {
			delete(cache.cacheItems, key)
		}
	}
	cache.mu.Unlock()

	// Only permanent items should remain
	assert.True(t, cache.Has("permanent1"))
	assert.False(t, cache.Has("temporary1"))
	assert.True(t, cache.Has("permanent2"))
	assert.False(t, cache.Has("temporary2"))
}
