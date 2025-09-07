package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetChannelCacheKey(t *testing.T) {
	testCases := []struct {
		name     string
		prefix   string
		channel  string
		expected string
	}{
		{"No prefix, regular channel", "", "my-channel", "app##channel#my-channel"},
		{"With prefix, regular channel", "myprefix", "my-channel", "app#myprefix#channel#my-channel"},
		{"No prefix, presence channel", "", "presence-my-channel", "app##channel#presence-my-channel"},
		{"With prefix, presence channel", "myprefix", "presence-my-channel", "app#myprefix#channel#presence-my-channel"},
		{"No prefix, private channel", "", "private-my-channel", "app##channel#private-my-channel"},
		{"With prefix, private channel", "myprefix", "private-my-channel", "app#myprefix#channel#private-my-channel"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetChannelCacheKey(tc.prefix, tc.channel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLocalCache(t *testing.T) {
	localCache := &LocalCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = localCache.Init(ctx)

	t.Run("SetExWithExpiration", func(t *testing.T) {
		// Test with expiration
		localCache.SetEx("keyex", "value-with-ttl", 500*time.Millisecond)

		// Should exist initially
		val, exists := localCache.Get("keyex")
		assert.True(t, exists)
		assert.Equal(t, "value-with-ttl", val)

		time.Sleep(600 * time.Millisecond)

		// Should be expired now
		val, exists = localCache.Get("keyex")
		assert.False(t, exists)
		assert.Equal(t, "", val)
	})

	cacheFactory := func() CacheContract {
		return localCache
	}
	RunCacheTests(t, "LocalCache", cacheFactory)
}

func TestRedisCache(t *testing.T) {
	redisCache := &RedisCache{
		Client: nil,
		Prefix: "test",
	}
	t.Run("InitEmpty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := redisCache.Init(ctx)
		assert.Error(t, err)
		assert.Equal(t, "redis Client is not initialized", err.Error())
	})

	// Test with valid client
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	redisCache = &RedisCache{
		Client: client,
		Prefix: "test",
	}

	t.Run("InitValid", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := redisCache.Init(ctx)
		assert.NoError(t, err)
	})

	t.Run("GetKey", func(t *testing.T) {
		key := redisCache.getKey("mykey")
		assert.Equal(t, "test:cache:mykey", key)
	})

	// need to test this here because we need the fastforward ability of the miniredis
	t.Run("SetExWithExpiration", func(t *testing.T) {
		// Test with expiration
		redisCache.SetEx("keyex", "value-with-ttl", time.Second)

		// Should exist initially
		val, exists := redisCache.Get("keyex")
		assert.True(t, exists)
		assert.Equal(t, "value-with-ttl", val)

		mr.FastForward(2 * time.Second)

		// Should be expired now
		val, exists = redisCache.Get("keyex")
		assert.False(t, exists)
		assert.Equal(t, "", val)
	})

	cacheFactory := func() CacheContract {
		return redisCache
	}
	RunCacheTests(t, "RedisCache", cacheFactory)
}

func RunCacheTests(t *testing.T, name string, factory func() CacheContract) {
	t.Run(name, func(t *testing.T) {
		cache := factory()

		t.Run("Get", func(t *testing.T) {
			key := "testKey"
			if result, exists := cache.Get(key); exists {
				t.Errorf("Expected key %s to not exist, got %s", key, result)
			}
			cache.Set(key, "testValue")
			if result, exists := cache.Get(key); !exists {
				t.Errorf("Expected key %s to exist, got %s", key, result)
			}
		})

		t.Run("SetAndGet", func(t *testing.T) {
			cache.Set("key1", "value1")
			val, exists := cache.Get("key1")
			assert.True(t, exists)
			assert.Equal(t, "value1", val)

			// Test non-existent key
			val, exists = cache.Get("nonexistent")
			assert.False(t, exists)
			assert.Equal(t, "", val)
		})

		t.Run("Delete", func(t *testing.T) {
			// Set a key to delete
			cache.Set("key-to-delete", "value")
			assert.True(t, cache.Has("key-to-delete"))

			// Delete the key
			cache.Delete("key-to-delete")

			// Verify it's gone
			assert.False(t, cache.Has("key-to-delete"))

			// Deleting non-existent key should not error
			cache.Delete("nonexistent")
		})

		t.Run("Has", func(t *testing.T) {
			// Test with non-existent key
			assert.False(t, cache.Has("key-not-exists"))

			// Test with existing key
			cache.Set("key-exists", "value")
			assert.True(t, cache.Has("key-exists"))
		})

		t.Run("Remember", func(t *testing.T) {
			// Test with cache miss (callback executed)
			callCount := 0
			callback := func() (string, error) {
				callCount++
				return "computed-value", nil
			}

			val, err := cache.Remember("key-remember", 60, callback)
			assert.NoError(t, err)
			assert.Equal(t, "computed-value", val)
			assert.Equal(t, 1, callCount)

			// Test with cache hit (callback not executed)
			val, err = cache.Remember("key-remember", 60, callback)
			assert.NoError(t, err)
			assert.Equal(t, "computed-value", val)
			assert.Equal(t, 1, callCount) // Should still be 1

			// Test with callback returning error
			errCallback := func() (string, error) {
				return "", errors.New("callback error")
			}

			val, err = cache.Remember("key-error", 60, errCallback)
			assert.Error(t, err)
			assert.Equal(t, "callback error", err.Error())
			assert.Equal(t, "", val)
		})

		t.Run("Update", func(t *testing.T) {
			// Test updating non-existent key (should do nothing)
			cache.Update("nonexistent", "value")
			assert.False(t, cache.Has("nonexistent"))

			// Test updating existing key
			cache.Set("key-update", "original-value")

			val, exists := cache.Get("key-update")
			assert.True(t, exists)
			assert.Equal(t, "original-value", val)

			cache.Update("key-update", "updated-value")

			val, exists = cache.Get("key-update")
			assert.True(t, exists)
			assert.Equal(t, "updated-value", val)
		})

		t.Run("ComplexValues", func(t *testing.T) {
			// Test with complex strings (JSON, special chars)
			jsonValue := `{"name":"test","values":[1,2,3],"nested":{"key":"value"}}`
			specialChars := "!@#$%^&*()_+<>?:\"{}|~`-=[]\\;',./✓"

			cache.Set("json-key", jsonValue)
			cache.Set("special-chars", specialChars)

			val1, exists := cache.Get("json-key")
			assert.True(t, exists)
			assert.Equal(t, jsonValue, val1)

			val2, exists := cache.Get("special-chars")
			assert.True(t, exists)
			assert.Equal(t, specialChars, val2)
		})

	})
}
