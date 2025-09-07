package util

import (
	"testing"

	"pusher/env"
	"pusher/internal/constants"

	"github.com/stretchr/testify/assert"
)

func TestFileExists(t *testing.T) {
	// Test existing file
	exists := FileExists("utils.go")
	assert.True(t, exists, "Expected file to exist")

	// Test non-existing file
	exists = FileExists("non_existing_file.go")
	assert.False(t, exists, "Expected file to not exist")
}

func TestGenerateSocketID(t *testing.T) {
	// Test multiple socket IDs to ensure they follow the pattern
	for i := 0; i < 5; i++ {
		socketID := GenerateSocketID()

		// Check that it's not empty
		assert.NotEmpty(t, socketID)

		// Check that it matches the expected format (number.number)
		matched := assert.Regexp(t, `^\d+\.\d+$`, string(socketID))
		assert.True(t, matched, "Socket ID should match the pattern number.number")
	}
}

func TestIsPresenceChannel(t *testing.T) {
	testCases := []struct {
		name     string
		channel  constants.ChannelName
		expected bool
	}{
		{"Regular presence channel", "presence-channel", true},
		{"Presence cache channel", "presence-cache-channel", true},
		{"Private channel", "private-channel", false},
		{"Public channel", "public-channel", false},
		{"Empty channel", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPresenceChannel(tc.channel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsPrivateChannel(t *testing.T) {
	testCases := []struct {
		name     string
		channel  constants.ChannelName
		expected bool
	}{
		{"Regular private channel", "private-channel", true},
		{"Private cache channel", "private-cache-channel", true},
		{"Presence channel", "presence-channel", false},
		{"Public channel", "public-channel", false},
		{"Empty channel", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPrivateChannel(tc.channel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsPrivateEncryptedChannel(t *testing.T) {
	testCases := []struct {
		name     string
		channel  constants.ChannelName
		expected bool
	}{
		{"Regular encrypted channel", "private-encrypted-channel", true},
		{"Encrypted cache channel", "private-encrypted-cache-channel", true},
		{"Private channel", "private-channel", false},
		{"Presence channel", "presence-channel", false},
		{"Public channel", "public-channel", false},
		{"Empty channel", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPrivateEncryptedChannel(tc.channel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsCacheChannel(t *testing.T) {
	testCases := []struct {
		name     string
		channel  constants.ChannelName
		expected bool
	}{
		{"Regular cache channel", "cache-channel", true},
		{"Presence cache channel", "presence-cache-channel", true},
		{"Private cache channel", "private-cache-channel", true},
		{"Encrypted cache channel", "private-encrypted-cache-channel", true},
		{"Private channel", "private-channel", false},
		{"Presence channel", "presence-channel", false},
		{"Public channel", "public-channel", false},
		{"Empty channel", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsCacheChannel(tc.channel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsClientEvent(t *testing.T) {
	testCases := []struct {
		name     string
		event    string
		expected bool
	}{
		{"Client event", "client-event", true},
		{"Client with dots", "client-my.event", true},
		{"Server event", "server-event", false},
		{"Regular event", "event", false},
		{"Empty event", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsClientEvent(tc.event)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidChannel(t *testing.T) {
	testCases := []struct {
		name     string
		channel  constants.ChannelName
		expected bool
	}{
		{"Server to user channel", "#server-to-user-123", true},
		{"Valid channel", "test-channel", true},
		{"Valid channel with underscore", "test_channel", true},
		{"Valid channel with numbers", "test123", true},
		{"Valid channel with special chars", "test-channel@,;=.", true},
		{"Too long channel name", constants.ChannelName(string(make([]byte, 165))), false},
		{"Invalid chars", "test%channel", false},
		{"Invalid chars space", "test channel", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidChannel(tc.channel, 50)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateChannelName(t *testing.T) {
	testCases := []struct {
		name        string
		channel     constants.ChannelName
		maxLength   int
		expectError bool
	}{
		{"Valid channel", "test-channel", 50, false},
		{"Valid channel with underscore", "test_channel", 50, false},
		{"Valid channel with numbers", "test123", 50, false},
		{"Valid channel with special chars", "test-channel@,;=.", 50, false},
		{"Too long channel name", constants.ChannelName(string(make([]byte, 165))), 164, true},
		{"Invalid chars", "test%channel", 50, true},
		{"Invalid chars space", "test channel", 50, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChannelName(tc.channel, tc.maxLength)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidAppID(t *testing.T) {
	testCases := []struct {
		name     string
		appID    string
		expected bool
	}{
		{"Valid numeric ID", "12345", true},
		{"Invalid with letters", "123abc", false},
		{"Invalid with special chars", "123-456", false},
		{"Empty ID", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidAppID(tc.appID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStr2Int64(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{"Valid number", "12345", 12345, false},
		{"Zero", "0", 0, false},
		{"Negative number", "-123", -123, false},
		{"Invalid input", "abc", 0, true},
		{"Empty string", "", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Str2Int64(tc.input)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestStr2Int(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
		hasError bool
	}{
		{"Valid number", "12345", 12345, false},
		{"Zero", "0", 0, false},
		{"Negative number", "-123", -123, false},
		{"Invalid input", "abc", 0, true},
		{"Empty string", "", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Str2Int(tc.input)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestValidateChannelAuth(t *testing.T) {
	// Save original env value and restore after tests
	originalSecret := env.GetString("APP_SECRET", "")
	appSecret := "test_secret"
	t.Setenv("APP_SECRET", appSecret)
	defer t.Setenv("APP_SECRET", originalSecret)

	socketID := constants.SocketID("1234.5678")
	channelName := constants.ChannelName("private-channel")

	// Valid signature for given input
	validSignature := HmacSignature("1234.5678:private-channel", appSecret)
	authToken := "app_key:" + validSignature

	t.Run("Valid auth", func(t *testing.T) {
		result := ValidateChannelAuth(authToken, appSecret, socketID, channelName, "")
		assert.True(t, result)
	})

	t.Run("Empty auth token", func(t *testing.T) {
		result := ValidateChannelAuth("", appSecret, socketID, channelName, "")
		assert.False(t, result)
	})

	t.Run("Invalid auth token format", func(t *testing.T) {
		result := ValidateChannelAuth("invalid_format", appSecret, socketID, channelName, "")
		assert.False(t, result)
	})

	t.Run("Empty key part", func(t *testing.T) {
		result := ValidateChannelAuth(":"+validSignature, appSecret, socketID, channelName, "")
		assert.False(t, result)
	})

	t.Run("Invalid signature", func(t *testing.T) {
		result := ValidateChannelAuth("app_key:invalid_signature", appSecret, socketID, channelName, "")
		assert.False(t, result)
	})

	t.Run("With channel data", func(t *testing.T) {
		channelData := `{"user_id":"1","user_info":{"name":"Test"}}`
		validSignatureWithData := HmacSignature("1234.5678:private-channel:"+channelData, "test_secret")
		authTokenWithData := "app_key:" + validSignatureWithData

		result := ValidateChannelAuth(authTokenWithData, appSecret, socketID, channelName, channelData)
		assert.True(t, result)
	})
}
