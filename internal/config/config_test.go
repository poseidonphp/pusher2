package config

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/util"

	"github.com/spf13/viper"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis creates a miniredis instance and client for testing
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

func resetViper() {
	viper.Reset()
}

func setupTestEnv(t *testing.T) {
	// Clear existing env vars that might interfere with tests
	_ = os.Unsetenv("APP_ID")
	_ = os.Unsetenv("APP_KEY")
	_ = os.Unsetenv("APP_SECRET")
	_ = os.Unsetenv("BIND_PORT")
	_ = os.Unsetenv("BIND_ADDR")
	_ = os.Unsetenv("APP_ENV")

	_ = os.Unsetenv("CHANNEL_CACHE_MANAGER")
	_ = os.Unsetenv("REDIS_PREFIX")
	_ = os.Unsetenv("APP_ACTIVITY_TIMEOUT")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("REDIS_URL")
	_ = os.Unsetenv("QUEUE_DRIVER")
	_ = os.Unsetenv("CACHE_DRIVER")
	_ = os.Unsetenv("ADAPTER_DRIVER")
	_ = os.Unsetenv("REDIS_TLS")
	_ = os.Unsetenv("REDIS_CLUSTER")
	_ = os.Unsetenv("TEST_WITH_ENV_FILE")

	// Set minimum required env vars
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")
}

func TestFileExists(t *testing.T) {
	// Test with existing file
	tempFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	assert.True(t, util.FileExists(tempFile.Name()), "Should return true for existing file")

	// Test with non-existing file
	assert.False(t, util.FileExists("/path/to/nonexistent/file"), "Should return false for non-existing file")
}

func TestInitializeServerConfig_DefaultValues(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	ctx := context.Background()
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)

	// Test default values
	assert.Equal(t, "production", config.Env)
	assert.Equal(t, "6001", config.Port)
	assert.Equal(t, "0.0.0.0", config.BindAddress)
	assert.Equal(t, "local", config.AdapterDriver)
	assert.Equal(t, "local", config.QueueDriver)
	assert.Equal(t, "local", config.ChannelCacheDriver)
	assert.Equal(t, "array", config.AppManager)
	assert.Equal(t, "localhost:6379", config.RedisUrl)
	assert.Equal(t, "pusher", config.RedisPrefix)
	assert.False(t, config.RedisTls)
	assert.False(t, config.RedisClusterMode)
	assert.Equal(t, "warn", config.LogLevel)
	assert.False(t, config.IgnoreLoggerMiddleware)
	assert.False(t, config.UsingRedis)
}

func TestInitializeServerConfig_WithEnvironmentVariables(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	// Set environment variables
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")

	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "8080")
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("ADAPTER_DRIVER", "redis")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("REDIS_URL", "redis://"+mr.Addr())
	t.Setenv("REDIS_PREFIX", "test-prefix")
	t.Setenv("REDIS_TLS", "false")
	t.Setenv("REDIS_CLUSTER", "false")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)

	// Test that environment variables are respected
	assert.Equal(t, "development", config.Env)
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "127.0.0.1", config.BindAddress)
	assert.Equal(t, "redis", config.AdapterDriver)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "redis://"+mr.Addr(), config.RedisUrl)
	assert.Equal(t, "test-prefix", config.RedisPrefix)
	assert.False(t, config.RedisTls)
	assert.False(t, config.RedisClusterMode)
	assert.True(t, config.UsingRedis)
}

func TestInitializeServerConfig_SingleAppMode(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Set single app configuration
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")
	t.Setenv("APP_ACTIVITY_TIMEOUT", "120")
	t.Setenv("APP_AUTHORIZATION_TIMEOUT_SECONDS", "10")
	t.Setenv("APP_MAX_CONNECTIONS", "1000")
	t.Setenv("APP_ENABLE_CLIENT_MESSAGES", "true")
	t.Setenv("APP_ENABLED", "true")
	t.Setenv("APP_MAX_PRESENCE_MEMBERS_PER_CHANNEL", "200")
	t.Setenv("APP_MAX_PRESENCE_MEMBER_SIZE_IN_KB", "20")
	t.Setenv("APP_MAX_CHANNEL_NAME_LENGTH", "300")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)
	require.Len(t, config.Applications, 1)

	app := config.Applications[0]
	assert.Equal(t, "12345", app.ID)
	assert.Equal(t, "test-key", app.Key)
	assert.Equal(t, "test-secret", app.Secret)
	assert.Equal(t, 120, app.ActivityTimeout)
	assert.Equal(t, 10*time.Second, app.AuthorizationTimeout)
	assert.Equal(t, -1, app.MaxReadRequestsPerSecond) // validate the default value is set when not specified via env
	assert.Equal(t, int64(1000), app.MaxConnections)
	assert.True(t, app.EnableClientMessages)
	assert.True(t, app.Enabled)
	assert.Equal(t, 200, app.MaxPresenceMembersPerChannel)
	assert.Equal(t, 20, app.MaxPresenceMemberSizeInKb)
	assert.Equal(t, 300, app.MaxChannelNameLength)

	// Test calculated read timeout (activity timeout * 10/9)
	durationAs64 := int64(math.Round((float64(120) * 10.0) / 9.0))
	expectedReadTimeout := time.Duration(durationAs64) * time.Second
	assert.Equal(t, expectedReadTimeout, app.ReadTimeout)
}

func TestInitializeServerConfig_WithConfigFile(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	setupTestEnv(t)

	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	// Create a temporary config file
	configContent := fmt.Sprintf(`
app_env: "staging"
port: "9000"
bind_address: "0.0.0.0"
adapter_driver: "redis"
queue_driver: "redis"
cache_driver: "redis"
redis_url: "redis://%s"
redis_prefix: "config-file-test"
log_level: "warn"
applications:
  - app_id: "67890"
    app_key: "config-key"
    app_secret: "config-secret"
    app_activity_timeout: 180
    app_max_connections: 500
    app_enable_client_messages: false
    app_enabled: true
`, mr.Addr())

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set config file path
	t.Setenv("CONFIG_FILE", configFile)

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)

	// Test config file values
	assert.Equal(t, "staging", config.Env)
	assert.Equal(t, "9000", config.Port)
	assert.Equal(t, "0.0.0.0", config.BindAddress)
	assert.Equal(t, "redis", config.AdapterDriver)
	assert.Equal(t, "redis", config.QueueDriver)
	assert.Equal(t, "redis", config.ChannelCacheDriver)
	assert.Equal(t, "redis://"+mr.Addr(), config.RedisUrl)
	assert.Equal(t, "config-file-test", config.RedisPrefix)
	assert.Equal(t, "warn", config.LogLevel)
	assert.True(t, config.UsingRedis)

	// Test applications from config file
	require.Len(t, config.Applications, 1)
	app := config.Applications[0]
	assert.Equal(t, "67890", app.ID)
	assert.Equal(t, "config-key", app.Key)
	assert.Equal(t, "config-secret", app.Secret)
	assert.Equal(t, 180, app.ActivityTimeout)
	assert.Equal(t, int64(500), app.MaxConnections)
	assert.False(t, app.EnableClientMessages)
	assert.True(t, app.Enabled)
}

func TestInitializeServerConfig_WithEnvFile(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	resetViper()
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	setupTestEnv(t)

	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	// Create a temporary .env file
	envContent := fmt.Sprintf(`
APP_ENV=testing
PORT=7000
BIND_ADDRESS=192.168.1.1
ADAPTER_DRIVER=redis
LOG_LEVEL=trace
REDIS_URL=redis://%s
REDIS_USE_TLS=false
REDIS_PREFIX=env-test
APP_ID=99999
APP_KEY=env-key
APP_SECRET=env-secret
`, mr.Addr())

	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	err := os.WriteFile(envFile, []byte(envContent), 0644)
	require.NoError(t, err)

	// Set env file path
	t.Setenv("ENV_FILE", envFile)
	t.Setenv("TEST_WITH_ENV_FILE", "true") // this will force the check for a env file

	os.Unsetenv("APP_ID") // ensure no conflict with existing env vars
	os.Unsetenv("APP_KEY")
	os.Unsetenv("APP_SECRET")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)

	// Test .env file values
	assert.Equal(t, "testing", config.Env)
	assert.Equal(t, "7000", config.Port)
	assert.Equal(t, "192.168.1.1", config.BindAddress)
	assert.Equal(t, "redis", config.AdapterDriver)
	assert.Equal(t, "trace", config.LogLevel)
	assert.Equal(t, "redis://"+mr.Addr(), config.RedisUrl)
	assert.Equal(t, "env-test", config.RedisPrefix)
	assert.True(t, config.UsingRedis)

	// Test single app from .env file
	require.Len(t, config.Applications, 1)
	app := config.Applications[0]
	assert.Equal(t, "99999", app.ID)
	assert.Equal(t, "env-key", app.Key)
	assert.Equal(t, "env-secret", app.Secret)
}

func TestInitializeServerConfig_RedisConfiguration(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	// Test Redis configuration detection
	testCases := []struct {
		name            string
		adapterDriver   string
		queueDriver     string
		cacheDriver     string
		expectedRedis   bool
		redisUrl        string
		expectHostError bool
	}{
		{"All local drivers", "local", "local", "local", false, mr.Addr(), false},
		{"Redis adapter only", "redis", "local", "local", true, mr.Addr(), false},
		{"Redis queue only", "local", "redis", "local", true, mr.Addr(), false},
		{"Redis cache only", "local", "local", "redis", true, mr.Addr(), false},
		{"All Redis drivers", "redis", "redis", "redis", true, mr.Addr(), false},
		{"Mixed drivers", "redis", "local", "redis", true, mr.Addr(), false},
		{"Invalid host", "redis", "local", "redis", true, "baddaddress:9876", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset pflag for each test
			pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
			setupTestEnv(t)

			t.Setenv("ADAPTER_DRIVER", tc.adapterDriver)
			t.Setenv("QUEUE_DRIVER", tc.queueDriver)
			t.Setenv("CACHE_DRIVER", tc.cacheDriver)
			t.Setenv("REDIS_URL", "redis://"+tc.redisUrl)

			ctx := context.Background()
			config, err := InitializeServerConfig(&ctx)

			if tc.expectHostError {
				require.Error(t, err)
				require.Nil(t, config)
				assert.Contains(t, err.Error(), "Error initializing Redis client")
			} else {
				require.NoError(t, err)
				require.NotNil(t, config)

				assert.Equal(t, tc.expectedRedis, config.UsingRedis,
					"UsingRedis should be %v for adapter=%s, queue=%s, cache=%s",
					tc.expectedRedis, tc.adapterDriver, tc.queueDriver, tc.cacheDriver)
			}

		})
	}
}

func TestInitializeServerConfig_InvalidConfigFile(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Set path to non-existent config file
	t.Setenv("CONFIG_FILE", "/path/to/nonexistent/config.yaml")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestInitializeServerConfig_MalformedConfigFile(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	configContent := fmt.Sprintf(`
xxxxxx
app_env: "staging"
applications:
  - app_id: "67890"
    app_key: "config-key"
    app_secret: "config-secret"
`)

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set config file path to the malformed file
	t.Setenv("CONFIG_FILE", configFile)

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestInitializeServerConfig_InvalidEnvFile(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	resetViper()
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	setupTestEnv(t)

	// Set path to non-existent .env file
	t.Setenv("ENV_FILE", "/path/to/nonexistent/.env")
	t.Setenv("TEST_WITH_ENV_FILE", "true") // this will force the check for a env file

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "error finding .env file")
}

func TestInitializeServerConfig_WebhookConfiguration(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Set webhook-related environment variables
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")
	t.Setenv("APP_REQUIRE_CHANNEL_AUTHORIZATION", "true")
	t.Setenv("APP_HAS_CLIENT_EVENT_WEBHOOKS", "true")
	t.Setenv("APP_HAS_CHANNEL_OCCUPIED_WEBHOOKS", "true")
	t.Setenv("APP_HAS_CHANNEL_VACATED_WEBHOOKS", "true")
	t.Setenv("APP_HAS_MEMBER_ADDED_WEBHOOKS", "true")
	t.Setenv("APP_HAS_MEMBER_REMOVED_WEBHOOKS", "true")
	t.Setenv("APP_HAS_CACHE_MISS_WEBHOOKS", "true")
	t.Setenv("APP_WEBHOOK_BATCHING_ENABLED", "true")
	t.Setenv("APP_WEBHOOKS_ENABLED", "true")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)
	require.Len(t, config.Applications, 1)

	app := config.Applications[0]
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

func TestInitializeServerConfig_EventLimitsConfiguration(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Set event limit environment variables
	t.Setenv("APP_ID", "12345")
	t.Setenv("APP_KEY", "test-key")
	t.Setenv("APP_SECRET", "test-secret")
	t.Setenv("APP_MAX_BACKEND_EVENTS_PER_SECOND", "100")
	t.Setenv("APP_MAX_CLIENT_EVENTS_PER_SECOND", "50")
	t.Setenv("APP_MAX_READ_REQUESTS_PER_SECOND", "200")
	t.Setenv("APP_MAX_EVENT_CHANNELS_AT_ONCE", "5")
	t.Setenv("APP_MAX_EVENT_NAME_LENGTH", "150")
	t.Setenv("APP_MAX_EVENT_PAYLOAD_IN_KB", "15")
	t.Setenv("APP_MAX_EVENT_BATCH_SIZE", "20")

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)
	require.Len(t, config.Applications, 1)

	app := config.Applications[0]
	assert.Equal(t, 100, app.MaxBackendEventsPerSecond)
	assert.Equal(t, 50, app.MaxClientEventsPerSecond)
	assert.Equal(t, 200, app.MaxReadRequestsPerSecond)
	assert.Equal(t, 5, app.MaxEventChannelsAtOnce)
	assert.Equal(t, 150, app.MaxEventNameLength)
	assert.Equal(t, 15, app.MaxEventPayloadInKb)
	assert.Equal(t, 20, app.MaxEventBatchSize)
}

func TestInitializeServerConfig_NoApplications(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	resetViper()
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	setupTestEnv(t)
	_ = os.Unsetenv("APP_ID")
	_ = os.Unsetenv("APP_KEY")
	_ = os.Unsetenv("APP_SECRET")

	// Don't set any app-related environment variables
	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no applications configured")
	require.Nil(t, config)

	// // Should have no applications when no app config is provided
	// assert.Len(t, config.Applications, 0)
}

func TestInitializeServerConfig_ProductionEnvironment(t *testing.T) {
	// Reset pflag to avoid conflicts between tests
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	setupTestEnv(t)

	t.Setenv("APP_ENV", constants.PRODUCTION)

	ctx := context.Background()
	config, err := InitializeServerConfig(&ctx)

	require.NoError(t, err)
	require.NotNil(t, config)

	assert.Equal(t, constants.PRODUCTION, config.Env)
}

func TestAppConfig_Structure(t *testing.T) {
	// Test AppConfig struct
	appConfig := apps.App{
		ID:     "12345",
		Key:    "test-key",
		Secret: "test-secret",
	}
	appConfig.SetMissingDefaults()

	assert.Equal(t, "12345", appConfig.ID)
	assert.Equal(t, "test-key", appConfig.Key)
	assert.Equal(t, "test-secret", appConfig.Secret)
}

func TestServerConfig_Structure(t *testing.T) {
	// Test ServerConfig struct fields
	config := &ServerConfig{
		Env:                    "test",
		Port:                   "8080",
		BindAddress:            "127.0.0.1",
		AppManager:             "array",
		AdapterDriver:          "local",
		QueueDriver:            "local",
		ChannelCacheDriver:     "local",
		RedisPrefix:            "test",
		LogLevel:               "debug",
		RedisUrl:               "localhost:6379",
		RedisTls:               false,
		RedisClusterMode:       false,
		IgnoreLoggerMiddleware: true,
		UsingRedis:             false,
	}

	assert.Equal(t, "test", config.Env)
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "127.0.0.1", config.BindAddress)
	assert.Equal(t, "array", config.AppManager)
	assert.Equal(t, "local", config.AdapterDriver)
	assert.Equal(t, "local", config.QueueDriver)
	assert.Equal(t, "local", config.ChannelCacheDriver)
	assert.Equal(t, "test", config.RedisPrefix)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "localhost:6379", config.RedisUrl)
	assert.False(t, config.RedisTls)
	assert.False(t, config.RedisClusterMode)
	assert.True(t, config.IgnoreLoggerMiddleware)
	assert.False(t, config.UsingRedis)
}
