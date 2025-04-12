package cache

//
//import (
//	"errors"
//	"github.com/stretchr/testify/assert"
//	"testing"
//	"time"
//)
//
//func TestLocalCacheInit(t *testing.T) {
//	cache := &LocalCache{}
//	err := cache.Init()
//	if err != nil {
//		t.Errorf("Expected no error on Init, got %v", err)
//	}
//	if cache.cacheItems == nil {
//		t.Error("Expected cacheItems to be initialized")
//	}
//}
//
//func TestLocalCacheSetAndGet(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Test basic set and get
//	cache.Set("key1", "value1")
//	val, exists := cache.Get("key1")
//	if !exists {
//		t.Error("Expected key1 to exist")
//	}
//	if val != "value1" {
//		t.Errorf("Expected value1, got %s", val)
//	}
//
//	// Test getting non-existent key
//	_, exists = cache.Get("nonexistent")
//	if exists {
//		t.Error("Expected nonexistent key to not exist")
//	}
//}
//
//func TestLocalCacheSetExWithExpiration(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Set with short expiration
//	cache.SetEx("key1", "value1", 500*time.Millisecond)
//
//	// Should exist immediately
//	val, exists := cache.Get("key1")
//	if !exists {
//		t.Error("Expected key1 to exist immediately after SetEx")
//	}
//	if val != "value1" {
//		t.Errorf("Expected value1, got %s", val)
//	}
//
//	// Wait for expiration
//	time.Sleep(1000 * time.Millisecond)
//
//	// Should be gone after expiration
//	_, exists2 := cache.Get("key1")
//	if exists2 {
//		t.Error("Expected key1 to be expired")
//	}
//}
//
//func TestLocalCacheDelete(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Set and then delete
//	cache.Set("key1", "value1")
//
//	// Fix the Delete method before running this test
//	// The current implementation creates an infinite recursion
//	// The test assumes the fixed implementation
//	cache.Delete("key1")
//
//	_, exists := cache.Get("key1")
//	if exists {
//		t.Error("Expected key1 to be deleted")
//	}
//
//	// Delete non-existent key should not cause errors
//	cache.Delete("nonexistent")
//}
//
//func TestLocalCacheHas(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Test with non-existent key
//	if cache.Has("key1") {
//		t.Error("Expected Has to return false for non-existent key")
//	}
//
//	// Test with existing key
//	cache.Set("key1", "value1")
//	if !cache.Has("key1") {
//		t.Error("Expected Has to return true for existing key")
//	}
//
//	// Test with expired key
//	cache.SetEx("key2", "value2", 100*time.Millisecond)
//	time.Sleep(200 * time.Millisecond)
//	if cache.Has("key2") {
//		t.Error("Expected Has to return false for expired key")
//	}
//}
//
//func TestLocalCacheUpdate(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Update non-existent key should do nothing
//	cache.Update("key1", "new-value")
//	assert.False(t, cache.Has("key1"))
//
//	// Update existing key should change the value
//	cache.Set("key1", "value1")
//	cache.Update("key1", "new-value")
//
//	val, exists := cache.Get("key1")
//	assert.True(t, exists)
//	assert.Equal(t, val, "new-value")
//}
//
//func TestLocalCacheRemember(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Test with callback that returns a value
//	callCount := 0
//	callback := func() (string, error) {
//		callCount++
//		return "computed-value", nil
//	}
//
//	// First call should execute callback
//	val, err := cache.Remember("key1", 60, callback)
//	if err != nil {
//		t.Errorf("Expected no error, got %v", err)
//	}
//	if val != "computed-value" {
//		t.Errorf("Expected computed-value, got %s", val)
//	}
//	if callCount != 1 {
//		t.Errorf("Expected callback to be called once, got %d", callCount)
//	}
//
//	// Second call should use cached value
//	val, err = cache.Remember("key1", 60, callback)
//	if err != nil {
//		t.Errorf("Expected no error on cached value, got %v", err)
//	}
//	if val != "computed-value" {
//		t.Errorf("Expected computed-value from cache, got %s", val)
//	}
//	if callCount != 1 {
//		t.Errorf("Expected callback to still be called only once, got %d", callCount)
//	}
//
//	// Test with callback that returns an error
//	errorCallback := func() (string, error) {
//		return "", errors.New("callback error")
//	}
//
//	_, err = cache.Remember("key2", 60, errorCallback)
//	if err == nil || err.Error() != "callback error" {
//		t.Errorf("Expected callback error, got %v", err)
//	}
//}
//
//func TestLocalCacheExpiration(t *testing.T) {
//	cache := &LocalCache{}
//	_ = cache.Init()
//
//	// Set with infinite TTL (nil expireAt)
//	cache.Set("key1", "value1")
//
//	// Should still exist after some time
//	time.Sleep(100 * time.Millisecond)
//	_, exists := cache.Get("key1")
//	if !exists {
//		t.Error("Expected key1 with nil expireAt to not expire")
//	}
//
//	// Test that expired items are deleted when accessed
//	expTime := time.Now().Add(100 * time.Millisecond)
//	cache.mu.Lock()
//	cache.cacheItems["key2"] = &localCacheItem{
//		data:     "value2",
//		expireAt: &expTime,
//	}
//	cache.mu.Unlock()
//
//	time.Sleep(200 * time.Millisecond)
//	_, exists = cache.Get("key2")
//	if exists {
//		t.Error("Expected key2 to be removed when accessed after expiration")
//	}
//}
