package cache

import "github.com/alicebob/miniredis/v2"

import (
	"context"
	"github.com/redis/go-redis/v9"
	"testing"
)

// setupMiniRedis creates a new miniredis server and returns it with a connected UniversalClient
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %v", err)
	}

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{mr.Addr()},
	})

	// Verify connection is working
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		t.Fatalf("Could not connect to miniredis: %v", err)
	}

	return mr, client
}

//
//func TestRedisCache_Init(t *testing.T) {
//	// Test with nil client
//	cache := &RedisCache{
//		Client: nil,
//		Prefix: "test",
//	}
//	err := cache.Init()
//	assert.Error(t, err)
//	assert.Equal(t, "redis Client is not initialized", err.Error())
//
//	// Test with valid client
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache = &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//	err = cache.Init()
//	assert.NoError(t, err)
//}
//
//func TestRedisCache_GetKey(t *testing.T) {
//	cache := &RedisCache{
//		Prefix: "test",
//	}
//
//	key := cache.getKey("mykey")
//	assert.Equal(t, "test:cache:mykey", key)
//}
//
//func TestRedisCache_SetAndGet(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test basic set and get
//	cache.Set("key1", "value1")
//	val, exists := cache.Get("key1")
//	assert.True(t, exists)
//	assert.Equal(t, "value1", val)
//
//	// Verify actual key in Redis
//	redisVal, err := client.Get(context.Background(), "test:cache:key1").Result()
//	assert.NoError(t, err)
//	assert.Equal(t, "value1", redisVal)
//
//	// Test non-existent key
//	val, exists = cache.Get("nonexistent")
//	assert.False(t, exists)
//	assert.Equal(t, "", val)
//}
//
//func TestRedisCache_SetEx(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test with expiration
//	cache.SetEx("keyex", "value-with-ttl", 500*time.Millisecond)
//
//	// Should exist initially
//	val, exists := cache.Get("keyex")
//	assert.True(t, exists)
//	assert.Equal(t, "value-with-ttl", val)
//
//	//// Verify TTL is set
//	//ttl := client.TTL("test:cache:keyex").Val()
//	//assert.True(t, ttl > 0)
//
//	// Fast forward time in miniredis
//	mr.FastForward(1 * time.Second)
//
//	// Should be expired now
//	val, exists = cache.Get("keyex")
//	assert.False(t, exists)
//	assert.Equal(t, "", val)
//}
//
//func TestRedisCache_Delete(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Set a key to delete
//	cache.Set("key-to-delete", "value")
//	assert.True(t, cache.Has("key-to-delete"))
//
//	// Delete the key
//	cache.Delete("key-to-delete")
//
//	// Verify it's gone
//	assert.False(t, cache.Has("key-to-delete"))
//
//	// Deleting non-existent key should not error
//	cache.Delete("nonexistent")
//}
//
//func TestRedisCache_Has(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test with non-existent key
//	assert.False(t, cache.Has("key-not-exists"))
//
//	// Test with existing key
//	cache.Set("key-exists", "value")
//	assert.True(t, cache.Has("key-exists"))
//
//	// Test with expired key
//	cache.SetEx("key-expired", "value", 100*time.Millisecond)
//	mr.FastForward(200 * time.Millisecond)
//	assert.False(t, cache.Has("key-expired"))
//}
//
//func TestRedisCache_Remember(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test with cache miss (callback executed)
//	callCount := 0
//	callback := func() (string, error) {
//		callCount++
//		return "computed-value", nil
//	}
//
//	val, err := cache.Remember("key-remember", 60, callback)
//	assert.NoError(t, err)
//	assert.Equal(t, "computed-value", val)
//	assert.Equal(t, 1, callCount)
//
//	// Test with cache hit (callback not executed)
//	val, err = cache.Remember("key-remember", 60, callback)
//	assert.NoError(t, err)
//	assert.Equal(t, "computed-value", val)
//	assert.Equal(t, 1, callCount) // Should still be 1
//
//	// Test with callback returning error
//	errCallback := func() (string, error) {
//		return "", errors.New("callback error")
//	}
//
//	val, err = cache.Remember("key-error", 60, errCallback)
//	assert.Error(t, err)
//	assert.Equal(t, "callback error", err.Error())
//	assert.Equal(t, "", val)
//}
//
//func TestRedisCache_Update(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test updating non-existent key (should do nothing)
//	cache.Update("nonexistent", "value")
//	assert.False(t, cache.Has("nonexistent"))
//
//	// Test updating existing key
//	cache.Set("key-update", "original-value")
//
//	val, exists := cache.Get("key-update")
//	assert.True(t, exists)
//	assert.Equal(t, "original-value", val)
//
//	cache.Update("key-update", "updated-value")
//
//	val, exists = cache.Get("key-update")
//	assert.True(t, exists)
//	assert.Equal(t, "updated-value", val)
//}
//
//func TestRedisCache_ComplexValues(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//	defer client.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Test with complex strings (JSON, special chars)
//	jsonValue := `{"name":"test","values":[1,2,3],"nested":{"key":"value"}}`
//	specialChars := "!@#$%^&*()_+<>?:\"{}|~`-=[]\\;',./✓"
//
//	cache.Set("json-key", jsonValue)
//	cache.Set("special-chars", specialChars)
//
//	val1, exists := cache.Get("json-key")
//	assert.True(t, exists)
//	assert.Equal(t, jsonValue, val1)
//
//	val2, exists := cache.Get("special-chars")
//	assert.True(t, exists)
//	assert.Equal(t, specialChars, val2)
//}
//
//func TestRedisCache_ClientError(t *testing.T) {
//	mr, client := setupMiniRedis(t)
//	defer mr.Close()
//
//	cache := &RedisCache{
//		Client: client,
//		Prefix: "test",
//	}
//
//	// Set a test key
//	cache.Set("test-key", "test-value")
//
//	// Close the client to simulate connection error
//	client.Close()
//
//	// Operations should handle errors gracefully
//	val, exists := cache.Get("test-key")
//	assert.False(t, exists)
//	assert.Equal(t, "", val)
//
//	assert.False(t, cache.Has("test-key"))
//
//	// These should not panic
//	cache.Set("new-key", "value")
//	cache.SetEx("new-key-ex", "value", time.Minute)
//	cache.Delete("some-key")
//	cache.Update("some-key", "value")
//
//	// Remember should return the error
//	returnedVal, err := cache.Remember("key", 60, func() (string, error) {
//		return "value", nil
//	})
//	assert.Nil(t, err)
//	assert.Equal(t, returnedVal, "value")
//
//	returnedVal, err = cache.Remember("key2", 60, func() (string, error) {
//		return "", errors.New("test error")
//	})
//
//	assert.Error(t, err)
//	// test that the error string is "test error"
//	assert.Equal(t, err.Error(), "test error")
//}
