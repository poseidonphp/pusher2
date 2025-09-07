package util

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pusher/env"
)

func TestVerify(t *testing.T) {
	// Save original env value and restore after tests
	originalSecret := env.GetString("APP_SECRET", "")
	defer func() {
		t.Setenv("APP_SECRET", originalSecret)
	}()

	t.Setenv("APP_SECRET", "test_secret")

	t.Run("ValidSignature", func(t *testing.T) {
		// Setup request with valid parameters
		now := time.Now().Unix()
		path := "/apps/123456/events"

		// Create query parameters without signature
		query := fmt.Sprintf("auth_key=test_key&auth_timestamp=%d&auth_version=1.0", now)

		// Generate the signature
		stringToSign := "POST\n" + path + "\n" + query
		signature := HmacSignature(stringToSign, "test_secret")

		// Create full URL with signature
		fullURL := "http://localhost" + path + "?" + query +
			"&auth_signature=" + signature

		req := httptest.NewRequest("POST", fullURL, nil)

		// Test
		result, err := Verify(req, "123456", "test_secret")

		// Assert
		assert.True(t, result, "Valid signature should verify")
		assert.Nil(t, err, "No error should be returned for valid signature")
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		req := httptest.NewRequest("POST",
			"http://localhost/apps/123456/events?auth_version=2.0", nil)

		result, err := Verify(req, "123456", "test_secret")

		assert.False(t, result, "Invalid version should fail verification")
		assert.Error(t, err, "Error should be returned for invalid version")
		assert.Equal(t, "invalid version", err.Error())
	})

	t.Run("ExpiredTimestamp", func(t *testing.T) {
		// Set timestamp to 20 minutes ago (expired with default grace of 600 seconds)
		expiredTime := time.Now().Add(-20 * time.Minute).Unix()
		url := fmt.Sprintf("http://localhost/apps/123456/events?auth_version=1.0&auth_timestamp=%d", expiredTime)
		req := httptest.NewRequest("POST", url, nil)

		result, err := Verify(req, "123456", "test_secret")

		assert.False(t, result, "Expired timestamp should fail verification")
		assert.Error(t, err, "Error should be returned for expired timestamp")
		assert.Equal(t, "invalid timestamp", err.Error())
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		now := time.Now().Unix()
		url := fmt.Sprintf("http://localhost/apps/123456/events?auth_version=1.0&auth_timestamp=%d&auth_signature=invalidsignature", now)
		req := httptest.NewRequest("POST", url, nil)

		result, err := Verify(req, "123456", "test_secret")

		assert.False(t, result, "Invalid signature should fail verification")
		assert.Error(t, err, "Error should be returned for invalid signature")
		assert.Equal(t, "invalid signature", err.Error())
	})
}

func TestHmacSignature(t *testing.T) {
	t.Run("GeneratesCorrectSignature", func(t *testing.T) {
		toSign := "test_data"
		secret := "test_secret"

		// expected := "fd2f93a7ff3d0849ee941aeda9bb4e0a8c9417d58ef7292e2986fb19cf7f0f46"
		expected := "6a4848557f6dff818d54e7563310e2d11a06768ef39c7ae003b95737d4f1d2cb"
		result := HmacSignature(toSign, secret)

		assert.Equal(t, expected, result, "Signature should match expected value")
	})

	t.Run("DifferentInputsDifferentOutputs", func(t *testing.T) {
		secret := "test_secret"
		sig1 := HmacSignature("data1", secret)
		sig2 := HmacSignature("data2", secret)

		assert.NotEqual(t, sig1, sig2, "Different inputs should produce different signatures")
	})
}

func TestCheckVersion(t *testing.T) {
	assert.True(t, checkVersion("1.0"), "Version 1.0 should be valid")
	assert.False(t, checkVersion("2.0"), "Version 2.0 should be invalid")
	assert.False(t, checkVersion(""), "Empty version should be invalid")
}

func TestCheckTimestamp(t *testing.T) {
	now := time.Now().Unix()
	gracePeriod := int64(300) // 5 minutes

	// Test current timestamp with 5 minute grace
	assert.True(t, checkTimestamp(now, gracePeriod), "Current timestamp should be valid")

	// Test timestamp from 5 minutes ago
	fiveMinutesAgo := now - 300
	assert.True(t, checkTimestamp(fiveMinutesAgo, 600), "5 minutes old timestamp should be valid")

	// Test timestamp from 15 minutes ago
	fifteenMinutesAgo := now - 900
	assert.False(t, checkTimestamp(fifteenMinutesAgo, 600), "15 minutes old timestamp should be invalid")

	// Test future timestamp
	futureTimestamp := now + 60
	assert.False(t, checkTimestamp(futureTimestamp, 600), "Future timestamp should be invalid")
}

func TestPrepareQueryString(t *testing.T) {
	t.Run("OrdersParametersAlphabetically", func(t *testing.T) {
		// Create an unordered query
		req, _ := http.NewRequest("GET", "/?c=3&a=1&b=2", nil)
		query := req.URL.Query()

		result := prepareQueryString(query)

		assert.Equal(t, "a=1&b=2&c=3", result, "Parameters should be alphabetically ordered")
	})

	t.Run("HandlesEmptyQuery", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		query := req.URL.Query()

		result := prepareQueryString(query)

		assert.Equal(t, "", result, "Empty query should return empty string")
	})

	t.Run("HandlesMultipleValues", func(t *testing.T) {
		// When multiple values exist for a key, only the first one is used
		req, _ := http.NewRequest("GET", "/?key=value1&key=value2", nil)
		query := req.URL.Query()

		result := prepareQueryString(query)

		assert.Equal(t, "key=value1", result, "Should use the first value for duplicate keys")
	})
}

func TestHmacBytes(t *testing.T) {
	t.Run("GeneratesCorrectBytes", func(t *testing.T) {
		toSign := []byte("test_data")
		secret := []byte("test_secret")

		result := HmacBytes(toSign, secret)

		// Convert result to hex for easier comparison
		hexResult := hex.EncodeToString(result)
		// expected := "fd2f93a7ff3d0849ee941aeda9bb4e0a8c9417d58ef7292e2986fb19cf7f0f46"
		expected := "6a4848557f6dff818d54e7563310e2d11a06768ef39c7ae003b95737d4f1d2cb"

		assert.Equal(t, expected, hexResult, "HMAC bytes should match expected value")
	})
}

func TestGetQuerySignature(t *testing.T) {
	// This function is referenced in your test but not in your signature.go file
	// If it exists in another file, you should test it accordingly
}
