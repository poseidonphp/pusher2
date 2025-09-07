package apps

import (
	"testing"
	"time"

	"pusher/internal/constants"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// APP STRUCT TESTS
// ============================================================================

func TestApp_StructCreation(t *testing.T) {
	app := App{
		ID:     "test-app",
		Key:    "test-key",
		Secret: "test-secret",
	}

	assert.Equal(t, constants.AppID("test-app"), app.ID)
	assert.Equal(t, "test-key", app.Key)
	assert.Equal(t, "test-secret", app.Secret)
}

func TestApp_StructWithAllFields(t *testing.T) {
	app := App{
		ID:                           "test-app",
		Key:                          "test-key",
		Secret:                       "test-secret",
		ActivityTimeout:              120,
		ReadTimeout:                  30 * time.Second,
		AuthorizationTimeoutSeconds:  10,
		AuthorizationTimeout:         10 * time.Second,
		MaxConnections:               1000,
		EnableClientMessages:         true,
		Enabled:                      true,
		MaxBackendEventsPerSecond:    100,
		MaxClientEventsPerSecond:     50,
		MaxReadRequestsPerSecond:     200,
		Webhooks:                     []constants.Webhook{},
		MaxPresenceMembersPerChannel: 200,
		MaxPresenceMemberSizeInKb:    20,
		MaxChannelNameLength:         150,
		MaxEventChannelsAtOnce:       15,
		MaxEventNameLength:           150,
		MaxEventPayloadInKb:          150,
		MaxEventBatchSize:            30,
		RequireChannelAuthorization:  true,
		HasClientEventWebhooks:       true,
		HasChannelOccupiedWebhooks:   true,
		HasChannelVacatedWebhooks:    true,
		HasMemberAddedWebhooks:       true,
		HasMemberRemovedWebhooks:     true,
		HasCacheMissWebhooks:         true,
		WebhookBatchingEnabled:       true,
		WebhooksEnabled:              true,
	}

	assert.Equal(t, constants.AppID("test-app"), app.ID)
	assert.Equal(t, "test-key", app.Key)
	assert.Equal(t, "test-secret", app.Secret)
	assert.Equal(t, 120, app.ActivityTimeout)
	assert.Equal(t, 30*time.Second, app.ReadTimeout)
	assert.Equal(t, 10, app.AuthorizationTimeoutSeconds)
	assert.Equal(t, 10*time.Second, app.AuthorizationTimeout)
	assert.Equal(t, int64(1000), app.MaxConnections)
	assert.True(t, app.EnableClientMessages)
	assert.True(t, app.Enabled)
	assert.Equal(t, 100, app.MaxBackendEventsPerSecond)
	assert.Equal(t, 50, app.MaxClientEventsPerSecond)
	assert.Equal(t, 200, app.MaxReadRequestsPerSecond)
	assert.Equal(t, 200, app.MaxPresenceMembersPerChannel)
	assert.Equal(t, 20, app.MaxPresenceMemberSizeInKb)
	assert.Equal(t, 150, app.MaxChannelNameLength)
	assert.Equal(t, 15, app.MaxEventChannelsAtOnce)
	assert.Equal(t, 150, app.MaxEventNameLength)
	assert.Equal(t, 150, app.MaxEventPayloadInKb)
	assert.Equal(t, 30, app.MaxEventBatchSize)
	assert.True(t, app.RequireChannelAuthorization)
	assert.True(t, app.HasClientEventWebhooks)
	assert.True(t, app.HasChannelOccupiedWebhooks)
	assert.True(t, app.HasChannelVacatedWebhooks)
	assert.True(t, app.HasMemberAddedWebhooks)
	assert.True(t, app.HasMemberRemovedWebhooks)
	assert.True(t, app.HasCacheMissWebhooks)
	assert.True(t, app.WebhookBatchingEnabled)
	assert.True(t, app.WebhooksEnabled)
}

// ============================================================================
// SETMISSINGDEFAULTS TESTS
// ============================================================================

func TestSetMissingDefaults_EmptyApp(t *testing.T) {
	app := App{}
	app.SetMissingDefaults()

	// Test that all defaults are set correctly
	assert.Equal(t, 60, app.ActivityTimeout)
	assert.Equal(t, 67*time.Second, app.ReadTimeout) // (60 * 10 / 9) = 66.67, rounded to 67
	assert.Equal(t, 5, app.AuthorizationTimeoutSeconds)
	assert.Equal(t, 5*time.Second, app.AuthorizationTimeout)
	assert.Equal(t, int64(-1), app.MaxConnections)
	assert.False(t, app.EnableClientMessages)
	assert.True(t, app.Enabled)
	assert.Equal(t, -1, app.MaxBackendEventsPerSecond)
	assert.Equal(t, -1, app.MaxClientEventsPerSecond)
	assert.Equal(t, -1, app.MaxReadRequestsPerSecond)
	assert.Equal(t, 100, app.MaxPresenceMembersPerChannel)
	assert.Equal(t, 10, app.MaxPresenceMemberSizeInKb)
	assert.Equal(t, 100, app.MaxChannelNameLength)
	assert.Equal(t, 10, app.MaxEventChannelsAtOnce)
	assert.Equal(t, 100, app.MaxEventNameLength)
	assert.Equal(t, 100, app.MaxEventPayloadInKb)
	assert.Equal(t, 20, app.MaxEventBatchSize)
	assert.False(t, app.RequireChannelAuthorization)
	assert.False(t, app.HasClientEventWebhooks)
	assert.False(t, app.HasChannelOccupiedWebhooks)
	assert.False(t, app.HasChannelVacatedWebhooks)
	assert.False(t, app.HasMemberAddedWebhooks)
	assert.False(t, app.HasMemberRemovedWebhooks)
	assert.False(t, app.HasCacheMissWebhooks)
	assert.False(t, app.WebhookBatchingEnabled)
	assert.False(t, app.WebhooksEnabled)

	// Test that Webhooks is initialized as empty slice
	assert.NotNil(t, app.Webhooks)
	assert.Equal(t, 0, len(app.Webhooks))
}

func TestSetMissingDefaults_PartialApp(t *testing.T) {
	app := App{
		ID:                           "test-app",
		Key:                          "test-key",
		Secret:                       "test-secret",
		ActivityTimeout:              120,
		MaxConnections:               500,
		EnableClientMessages:         true,
		MaxPresenceMembersPerChannel: 200,
		RequireChannelAuthorization:  true,
	}
	app.SetMissingDefaults()

	// Test that provided values are preserved
	assert.Equal(t, constants.AppID("test-app"), app.ID)
	assert.Equal(t, "test-key", app.Key)
	assert.Equal(t, "test-secret", app.Secret)
	assert.Equal(t, 120, app.ActivityTimeout)
	assert.Equal(t, int64(500), app.MaxConnections)
	assert.True(t, app.EnableClientMessages)
	assert.Equal(t, 200, app.MaxPresenceMembersPerChannel)
	assert.True(t, app.RequireChannelAuthorization)

	// Test that defaults are set for unset values
	assert.Equal(t, 133*time.Second, app.ReadTimeout) // (120 * 10 / 9) = 133.33, rounded to 133
	assert.Equal(t, 5, app.AuthorizationTimeoutSeconds)
	assert.Equal(t, 5*time.Second, app.AuthorizationTimeout)
	assert.True(t, app.Enabled)
	assert.Equal(t, -1, app.MaxBackendEventsPerSecond)
	assert.Equal(t, -1, app.MaxClientEventsPerSecond)
	assert.Equal(t, -1, app.MaxReadRequestsPerSecond)
	assert.Equal(t, 10, app.MaxPresenceMemberSizeInKb)
	assert.Equal(t, 100, app.MaxChannelNameLength)
	assert.Equal(t, 10, app.MaxEventChannelsAtOnce)
	assert.Equal(t, 100, app.MaxEventNameLength)
	assert.Equal(t, 100, app.MaxEventPayloadInKb)
	assert.Equal(t, 20, app.MaxEventBatchSize)
	assert.False(t, app.HasClientEventWebhooks)
	assert.False(t, app.HasChannelOccupiedWebhooks)
	assert.False(t, app.HasChannelVacatedWebhooks)
	assert.False(t, app.HasMemberAddedWebhooks)
	assert.False(t, app.HasMemberRemovedWebhooks)
	assert.False(t, app.HasCacheMissWebhooks)
	assert.False(t, app.WebhookBatchingEnabled)
	assert.False(t, app.WebhooksEnabled)
}

func TestSetMissingDefaults_ReadTimeoutCalculation(t *testing.T) {
	tests := []struct {
		name                string
		activityTimeout     int
		expectedReadTimeout time.Duration
	}{
		{
			name:                "30 seconds activity timeout",
			activityTimeout:     30,
			expectedReadTimeout: 33 * time.Second, // (30 * 10 / 9) = 33.33, rounded to 33
		},
		{
			name:                "60 seconds activity timeout",
			activityTimeout:     60,
			expectedReadTimeout: 67 * time.Second, // (60 * 10 / 9) = 66.67, rounded to 67
		},
		{
			name:                "120 seconds activity timeout",
			activityTimeout:     120,
			expectedReadTimeout: 133 * time.Second, // (120 * 10 / 9) = 133.33, rounded to 133
		},
		{
			name:                "300 seconds activity timeout",
			activityTimeout:     300,
			expectedReadTimeout: 333 * time.Second, // (300 * 10 / 9) = 333.33, rounded to 333
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				ActivityTimeout: tt.activityTimeout,
			}
			app.SetMissingDefaults()

			assert.Equal(t, tt.expectedReadTimeout, app.ReadTimeout)
		})
	}
}

func TestSetMissingDefaults_AuthorizationTimeoutCalculation(t *testing.T) {
	tests := []struct {
		name                         string
		authorizationTimeoutSeconds  int
		expectedAuthorizationTimeout time.Duration
	}{
		{
			name:                         "5 seconds authorization timeout",
			authorizationTimeoutSeconds:  5,
			expectedAuthorizationTimeout: 5 * time.Second,
		},
		{
			name:                         "10 seconds authorization timeout",
			authorizationTimeoutSeconds:  10,
			expectedAuthorizationTimeout: 10 * time.Second,
		},
		{
			name:                         "30 seconds authorization timeout",
			authorizationTimeoutSeconds:  30,
			expectedAuthorizationTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				AuthorizationTimeoutSeconds: tt.authorizationTimeoutSeconds,
			}
			app.SetMissingDefaults()

			assert.Equal(t, tt.authorizationTimeoutSeconds, app.AuthorizationTimeoutSeconds)
			assert.Equal(t, tt.expectedAuthorizationTimeout, app.AuthorizationTimeout)
		})
	}
}

func TestSetMissingDefaults_WebhooksInitialization(t *testing.T) {
	t.Run("nil webhooks", func(t *testing.T) {
		app := App{}
		app.SetMissingDefaults()

		assert.NotNil(t, app.Webhooks)
		assert.Equal(t, 0, len(app.Webhooks))
	})

	t.Run("existing webhooks", func(t *testing.T) {
		existingWebhooks := []constants.Webhook{
			{URL: "http://example.com/webhook1"},
			{URL: "http://example.com/webhook2"},
		}
		app := App{
			Webhooks: existingWebhooks,
		}
		app.SetMissingDefaults()

		assert.Equal(t, existingWebhooks, app.Webhooks)
		assert.Equal(t, 2, len(app.Webhooks))
	})
}

func TestSetMissingDefaults_UnlimitedValues(t *testing.T) {
	app := App{}
	app.SetMissingDefaults()

	// Test that unlimited values are set to -1
	assert.Equal(t, int64(-1), app.MaxConnections)
	assert.Equal(t, -1, app.MaxBackendEventsPerSecond)
	assert.Equal(t, -1, app.MaxClientEventsPerSecond)
	assert.Equal(t, -1, app.MaxReadRequestsPerSecond)
}

func TestSetMissingDefaults_ZeroValues(t *testing.T) {
	app := App{
		ActivityTimeout:              0,
		AuthorizationTimeoutSeconds:  0,
		MaxConnections:               0,
		MaxBackendEventsPerSecond:    0,
		MaxClientEventsPerSecond:     0,
		MaxReadRequestsPerSecond:     0,
		MaxPresenceMembersPerChannel: 0,
		MaxPresenceMemberSizeInKb:    0,
		MaxChannelNameLength:         0,
		MaxEventChannelsAtOnce:       0,
		MaxEventNameLength:           0,
		MaxEventPayloadInKb:          0,
		MaxEventBatchSize:            0,
	}
	app.SetMissingDefaults()

	// Test that zero values are replaced with defaults
	assert.Equal(t, 60, app.ActivityTimeout)
	assert.Equal(t, 5, app.AuthorizationTimeoutSeconds)
	assert.Equal(t, int64(-1), app.MaxConnections)
	assert.Equal(t, -1, app.MaxBackendEventsPerSecond)
	assert.Equal(t, -1, app.MaxClientEventsPerSecond)
	assert.Equal(t, -1, app.MaxReadRequestsPerSecond)
	assert.Equal(t, 100, app.MaxPresenceMembersPerChannel)
	assert.Equal(t, 10, app.MaxPresenceMemberSizeInKb)
	assert.Equal(t, 100, app.MaxChannelNameLength)
	assert.Equal(t, 10, app.MaxEventChannelsAtOnce)
	assert.Equal(t, 100, app.MaxEventNameLength)
	assert.Equal(t, 100, app.MaxEventPayloadInKb)
	assert.Equal(t, 20, app.MaxEventBatchSize)
}

// ============================================================================
// GETVALUEORFALLBACK TESTS
// ============================================================================

func TestGetValueOrFallback_String(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "empty string returns fallback",
			value:    "",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "non-empty string returns value",
			value:    "test",
			fallback: "default",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueOrFallback_Int(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		fallback int
		expected int
	}{
		{
			name:     "zero returns fallback",
			value:    0,
			fallback: 100,
			expected: 100,
		},
		{
			name:     "non-zero returns value",
			value:    50,
			fallback: 100,
			expected: 50,
		},
		{
			name:     "negative value returns value",
			value:    -1,
			fallback: 100,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueOrFallback_Int64(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		fallback int64
		expected int64
	}{
		{
			name:     "zero returns fallback",
			value:    0,
			fallback: 1000,
			expected: 1000,
		},
		{
			name:     "non-zero returns value",
			value:    500,
			fallback: 1000,
			expected: 500,
		},
		{
			name:     "negative value returns value",
			value:    -1,
			fallback: 1000,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueOrFallback_Bool(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		fallback bool
		expected bool
	}{
		{
			name:     "false returns fallback",
			value:    false,
			fallback: true,
			expected: true,
		},
		{
			name:     "true returns value",
			value:    true,
			fallback: false,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueOrFallback_Duration(t *testing.T) {
	tests := []struct {
		name     string
		value    time.Duration
		fallback time.Duration
		expected time.Duration
	}{
		{
			name:     "zero duration returns fallback",
			value:    0,
			fallback: 30 * time.Second,
			expected: 30 * time.Second,
		},
		{
			name:     "non-zero duration returns value",
			value:    60 * time.Second,
			fallback: 30 * time.Second,
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// EDGE CASES AND INTEGRATION TESTS
// ============================================================================

func TestSetMissingDefaults_MultipleCalls(t *testing.T) {
	app := App{}

	// First call
	app.SetMissingDefaults()
	originalReadTimeout := app.ReadTimeout
	originalMaxConnections := app.MaxConnections

	// Second call should not change values
	app.SetMissingDefaults()

	assert.Equal(t, originalReadTimeout, app.ReadTimeout)
	assert.Equal(t, originalMaxConnections, app.MaxConnections)
}

func TestSetMissingDefaults_AllBooleanFields(t *testing.T) {
	app := App{}
	app.SetMissingDefaults()

	// All boolean fields should default to false except Enabled
	assert.False(t, app.EnableClientMessages)
	assert.True(t, app.Enabled) // This is the only boolean that defaults to true
	assert.False(t, app.RequireChannelAuthorization)
	assert.False(t, app.HasClientEventWebhooks)
	assert.False(t, app.HasChannelOccupiedWebhooks)
	assert.False(t, app.HasChannelVacatedWebhooks)
	assert.False(t, app.HasMemberAddedWebhooks)
	assert.False(t, app.HasMemberRemovedWebhooks)
	assert.False(t, app.HasCacheMissWebhooks)
	assert.False(t, app.WebhookBatchingEnabled)
	assert.False(t, app.WebhooksEnabled)
}

func TestSetMissingDefaults_AllBooleanFieldsTrue(t *testing.T) {
	app := App{
		EnableClientMessages:        true,
		RequireChannelAuthorization: true,
		HasClientEventWebhooks:      true,
		HasChannelOccupiedWebhooks:  true,
		HasChannelVacatedWebhooks:   true,
		HasMemberAddedWebhooks:      true,
		HasMemberRemovedWebhooks:    true,
		HasCacheMissWebhooks:        true,
		WebhookBatchingEnabled:      true,
		WebhooksEnabled:             true,
	}
	app.SetMissingDefaults()

	// All boolean fields should remain true
	assert.True(t, app.EnableClientMessages)
	assert.True(t, app.RequireChannelAuthorization)
	assert.True(t, app.HasClientEventWebhooks)
	assert.True(t, app.HasChannelOccupiedWebhooks)
	assert.True(t, app.HasChannelVacatedWebhooks)
	assert.True(t, app.HasMemberAddedWebhooks)
	assert.True(t, app.HasMemberRemovedWebhooks)
	assert.True(t, app.HasCacheMissWebhooks)
	assert.True(t, app.WebhookBatchingEnabled)
	assert.True(t, app.WebhooksEnabled)
}
