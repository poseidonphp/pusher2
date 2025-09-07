package env

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// LOOKUP FUNCTION TESTS
// ============================================================================

func TestLookup_ExistingVariable(t *testing.T) {
	// Set a test environment variable
	key := "TEST_LOOKUP_VAR"
	expectedValue := "test_value_123"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Lookup with existing variable
	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)
}

func TestLookup_NonExistentVariable(t *testing.T) {
	// Test Lookup with non-existent variable
	key := "NON_EXISTENT_VAR_12345"
	value, exists := Lookup(key)
	assert.False(t, exists)
	assert.Equal(t, "", value)
}

func TestLookup_EmptyVariable(t *testing.T) {
	// Set an empty environment variable
	key := "TEST_EMPTY_VAR"
	os.Setenv(key, "")
	defer os.Unsetenv(key)

	// Test Lookup with empty variable
	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, "", value)
}

func TestLookup_SpecialCharacters(t *testing.T) {
	// Set environment variable with special characters
	key := "TEST_SPECIAL_VAR"
	expectedValue := "test@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Lookup with special characters
	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)
}

func TestLookup_UnicodeCharacters(t *testing.T) {
	// Set environment variable with unicode characters
	key := "TEST_UNICODE_VAR"
	expectedValue := "测试🚀🎉中文"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Lookup with unicode characters
	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)
}

// ============================================================================
// GET FUNCTION TESTS
// ============================================================================

func TestGet_ExistingVariable(t *testing.T) {
	// Set a test environment variable
	key := "TEST_GET_VAR"
	expectedValue := "test_value_456"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Get with existing variable
	value := Get(key)
	assert.Equal(t, expectedValue, value)
}

func TestGet_NonExistentVariable(t *testing.T) {
	// Test Get with non-existent variable
	key := "NON_EXISTENT_GET_VAR_12345"
	value := Get(key)
	assert.Equal(t, "", value)
}

func TestGet_WithFallback_ExistingVariable(t *testing.T) {
	// Set a test environment variable
	key := "TEST_GET_FALLBACK_EXISTING"
	expectedValue := "env_value"
	fallbackValue := "fallback_value"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Get with existing variable and fallback
	value := Get(key, fallbackValue)
	assert.Equal(t, expectedValue, value) // Should return env value, not fallback
}

func TestGet_WithFallback_NonExistentVariable(t *testing.T) {
	// Test Get with non-existent variable and fallback
	key := "NON_EXISTENT_GET_FALLBACK_12345"
	fallbackValue := "fallback_value_789"
	value := Get(key, fallbackValue)
	assert.Equal(t, fallbackValue, value)
}

func TestGet_WithMultipleFallbacks(t *testing.T) {
	// Test Get with multiple fallback values (should use first one)
	key := "NON_EXISTENT_MULTIPLE_FALLBACK_12345"
	fallback1 := "first_fallback"
	fallback2 := "second_fallback"
	value := Get(key, fallback1, fallback2)
	assert.Equal(t, fallback1, value)
}

func TestGet_EmptyVariable(t *testing.T) {
	// Set an empty environment variable
	key := "TEST_GET_EMPTY_VAR"
	os.Setenv(key, "")
	defer os.Unsetenv(key)

	// Test Get with empty variable
	value := Get(key)
	assert.Equal(t, "", value)
}

func TestGet_EmptyVariableWithFallback(t *testing.T) {
	// Set an empty environment variable
	key := "TEST_GET_EMPTY_FALLBACK_VAR"
	os.Setenv(key, "")
	defer os.Unsetenv(key)

	// Test Get with empty variable and fallback
	fallbackValue := "fallback_for_empty"
	value := Get(key, fallbackValue)
	assert.Equal(t, "", value) // Should return empty env value, not fallback
}

func TestGet_SpecialCharacters(t *testing.T) {
	// Set environment variable with special characters
	key := "TEST_GET_SPECIAL_VAR"
	expectedValue := "test@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Get with special characters
	value := Get(key)
	assert.Equal(t, expectedValue, value)
}

func TestGet_UnicodeCharacters(t *testing.T) {
	// Set environment variable with unicode characters
	key := "TEST_GET_UNICODE_VAR"
	expectedValue := "测试🚀🎉中文"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Get with unicode characters
	value := Get(key)
	assert.Equal(t, expectedValue, value)
}

// ============================================================================
// GETSTRING FUNCTION TESTS
// ============================================================================

func TestGetString_ExistingVariable(t *testing.T) {
	// Set a test environment variable
	key := "TEST_GETSTRING_VAR"
	expectedValue := "test_value_789"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test GetString with existing variable
	value := GetString(key)
	assert.Equal(t, expectedValue, value)
}

func TestGetString_NonExistentVariable(t *testing.T) {
	// Test GetString with non-existent variable
	key := "NON_EXISTENT_GETSTRING_VAR_12345"
	value := GetString(key)
	assert.Equal(t, "", value)
}

func TestGetString_WithFallback_ExistingVariable(t *testing.T) {
	// Set a test environment variable
	key := "TEST_GETSTRING_FALLBACK_EXISTING"
	expectedValue := "env_value_string"
	fallbackValue := "fallback_value_string"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test GetString with existing variable and fallback
	value := GetString(key, fallbackValue)
	assert.Equal(t, expectedValue, value) // Should return env value, not fallback
}

func TestGetString_WithFallback_NonExistentVariable(t *testing.T) {
	// Test GetString with non-existent variable and fallback
	key := "NON_EXISTENT_GETSTRING_FALLBACK_12345"
	fallbackValue := "fallback_value_string_789"
	value := GetString(key, fallbackValue)
	assert.Equal(t, fallbackValue, value)
}

func TestGetString_WithMultipleFallbacks(t *testing.T) {
	// Test GetString with multiple fallback values (should use first one)
	key := "NON_EXISTENT_GETSTRING_MULTIPLE_FALLBACK_12345"
	fallback1 := "first_fallback_string"
	fallback2 := "second_fallback_string"
	value := GetString(key, fallback1, fallback2)
	assert.Equal(t, fallback1, value)
}

func TestGetString_EmptyVariable(t *testing.T) {
	// Set an empty environment variable
	key := "TEST_GETSTRING_EMPTY_VAR"
	os.Setenv(key, "")
	defer os.Unsetenv(key)

	// Test GetString with empty variable
	value := GetString(key)
	assert.Equal(t, "", value)
}

func TestGetString_EmptyVariableWithFallback(t *testing.T) {
	// Set an empty environment variable
	key := "TEST_GETSTRING_EMPTY_FALLBACK_VAR"
	os.Setenv(key, "")
	defer os.Unsetenv(key)

	// Test GetString with empty variable and fallback
	fallbackValue := "fallback_for_empty_string"
	value := GetString(key, fallbackValue)
	assert.Equal(t, "", value) // Should return empty env value, not fallback
}

func TestGetString_SpecialCharacters(t *testing.T) {
	// Set environment variable with special characters
	key := "TEST_GETSTRING_SPECIAL_VAR"
	expectedValue := "test@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test GetString with special characters
	value := GetString(key)
	assert.Equal(t, expectedValue, value)
}

func TestGetString_UnicodeCharacters(t *testing.T) {
	// Set environment variable with unicode characters
	key := "TEST_GETSTRING_UNICODE_VAR"
	expectedValue := "测试🚀🎉中文"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test GetString with unicode characters
	value := GetString(key)
	assert.Equal(t, expectedValue, value)
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestLookupAndGetConsistency(t *testing.T) {
	// Test that Lookup and Get return consistent results
	key := "TEST_CONSISTENCY_VAR"
	expectedValue := "consistency_test_value"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Lookup
	lookupValue, lookupExists := Lookup(key)
	assert.True(t, lookupExists)
	assert.Equal(t, expectedValue, lookupValue)

	// Test Get
	getValue := Get(key)
	assert.Equal(t, expectedValue, getValue)

	// Values should be the same
	assert.Equal(t, lookupValue, getValue)
}

func TestGetAndGetStringEquivalence(t *testing.T) {
	// Test that Get and GetString return the same results
	key := "TEST_EQUIVALENCE_VAR"
	expectedValue := "equivalence_test_value"
	fallbackValue := "fallback_equivalence_value"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test with existing variable
	getValue := Get(key, fallbackValue)
	getStringValue := GetString(key, fallbackValue)
	assert.Equal(t, getValue, getStringValue)
	assert.Equal(t, expectedValue, getValue)

	// Test with non-existent variable
	nonExistentKey := "NON_EXISTENT_EQUIVALENCE_12345"
	getValue2 := Get(nonExistentKey, fallbackValue)
	getStringValue2 := GetString(nonExistentKey, fallbackValue)
	assert.Equal(t, getValue2, getStringValue2)
	assert.Equal(t, fallbackValue, getValue2)
}

func TestFunctionChaining(t *testing.T) {
	// Test that functions can be chained or used together
	key := "TEST_CHAINING_VAR"
	expectedValue := "chaining_test_value"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	// Test Lookup -> Get pattern
	_, exists := Lookup(key)
	if exists {
		result := Get(key, "fallback")
		assert.Equal(t, expectedValue, result)
	} else {
		t.Error("Expected variable to exist")
	}

	// Test GetString with conditional logic
	result := GetString(key, "fallback")
	if result != "fallback" {
		assert.Equal(t, expectedValue, result)
	}
}

// ============================================================================
// EDGE CASES AND ERROR CONDITIONS
// ============================================================================

func TestEmptyKey(t *testing.T) {
	// Test with empty key
	value, exists := Lookup("")
	assert.False(t, exists)
	assert.Equal(t, "", value)

	getValue := Get("")
	assert.Equal(t, "", getValue)

	getStringValue := GetString("")
	assert.Equal(t, "", getStringValue)
}

func TestEmptyKeyWithFallback(t *testing.T) {
	// Test with empty key and fallback
	fallbackValue := "fallback_for_empty_key"
	getValue := Get("", fallbackValue)
	assert.Equal(t, fallbackValue, getValue) // Empty key should use fallback

	getStringValue := GetString("", fallbackValue)
	assert.Equal(t, fallbackValue, getStringValue) // Empty key should use fallback
}

func TestLongKey(t *testing.T) {
	// Test with reasonably long key
	longKey := "VERY_LONG_KEY_12345678901234567890123456789012345678901234567890"
	expectedValue := "long_key_value"
	os.Setenv(longKey, expectedValue)
	defer os.Unsetenv(longKey)

	value, exists := Lookup(longKey)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)
}

func TestLongValue(t *testing.T) {
	// Test with reasonably long value
	key := "LONG_VALUE_KEY"
	longValue := "A" + strings.Repeat("B", 100) // 101 characters
	os.Setenv(key, longValue)
	defer os.Unsetenv(key)

	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, longValue, value)
}

func TestNumericStringValues(t *testing.T) {
	// Test with numeric string values
	key := "NUMERIC_STRING_VAR"
	expectedValue := "12345"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)

	getValue := Get(key)
	assert.Equal(t, expectedValue, getValue)
}

func TestBooleanStringValues(t *testing.T) {
	// Test with boolean string values
	key := "BOOLEAN_STRING_VAR"
	expectedValue := "true"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	value, exists := Lookup(key)
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)

	getValue := Get(key)
	assert.Equal(t, expectedValue, getValue)
}
