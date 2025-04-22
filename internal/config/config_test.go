package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pusher/internal/constants"
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

func setupTestEnv(t *testing.T) {
	// Clear existing env vars that might interfere with tests
	os.Unsetenv("APP_ID")
	os.Unsetenv("APP_KEY")
	os.Unsetenv("APP_SECRET")
	os.Unsetenv("BIND_PORT")
	os.Unsetenv("BIND_ADDR")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("WEBHOOK_MANAGER")
	os.Unsetenv("WEBHOOK_URL")
	os.Unsetenv("WEBHOOK_ENABLED")
	os.Unsetenv("DISPATCHER_MANAGER")
	os.Unsetenv("PUBSUB_MANAGER")
	os.Unsetenv("STORAGE_MANAGER")
	os.Unsetenv("CHANNEL_CACHE_MANAGER")
	os.Unsetenv("REDIS_PREFIX")
	os.Unsetenv("ACTIVITY_TIMEOUT")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("REDIS_URL")

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

	assert.True(t, fileExists(tempFile.Name()), "Should return true for existing file")

	// Test with non-existing file
	assert.False(t, fileExists("/path/to/nonexistent/file"), "Should return false for non-existing file")
}

func TestInitializeServerConfig(t *testing.T) {
	ctx := context.Background()
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	t.Run("ParseFlags", func(t *testing.T) {
		setupTestEnv(t)

		flags := ParseCommandLineFlags()

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "6001", config.Port)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		setupTestEnv(t)

		// Empty command line flags
		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Check default values
		assert.Equal(t, "6001", config.Port)
		assert.Equal(t, "0.0.0.0", config.BindAddress)
		assert.Equal(t, constants.PRODUCTION, config.Env)
		assert.Equal(t, "http", config.WebhookDriver)
		assert.Equal(t, false, config.WebhookEnabled)
		assert.Equal(t, "local", config.QueueDriver)
		assert.Equal(t, "local", config.PubSubDriver)
		assert.Equal(t, "local", config.StorageDriver)
		assert.Equal(t, "local", config.ChannelCacheDriver)
		assert.Equal(t, "pusher", config.RedisPrefix)
		assert.Equal(t, 60*time.Second, config.ActivityTimeout)
		assert.Equal(t, int64(100), config.MaxPresenceUsers)
		assert.Equal(t, 10*1024, config.MaxPresenceUserDataBytes)

		// Check app config
		assert.Len(t, config.Apps, 1)
		app, ok := config.Apps["12345"]
		assert.True(t, ok)
		assert.Equal(t, "12345", app.AppID)
		assert.Equal(t, "test-key", app.AppKey)
		assert.Equal(t, "test-secret", app.AppSecret)
	})

	t.Run("CustomConfig", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("BIND_PORT", "8080")
		t.Setenv("BIND_ADDR", "127.0.0.1")
		t.Setenv("APP_ENV", "development")
		t.Setenv("ACTIVITY_TIMEOUT", "120")
		t.Setenv("MAX_PRESENCE_USERS", "200")
		t.Setenv("MAX_PRESENCE_USER_DATA_KB", "20")

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Check custom values
		assert.Equal(t, "8080", config.Port)
		assert.Equal(t, "127.0.0.1", config.BindAddress)
		assert.Equal(t, "development", config.Env)
		assert.Equal(t, 120*time.Second, config.ActivityTimeout)
		assert.Equal(t, int64(200), config.MaxPresenceUsers)
		assert.Equal(t, 20*1024, config.MaxPresenceUserDataBytes)
	})

	t.Run("CommandLinePortOverride", func(t *testing.T) {
		setupTestEnv(t)

		// Set command line flag to override env
		flags := CommandLineFlags{
			Port: "9000",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.Equal(t, "9000", config.Port)
	})

	t.Run("InvalidAppConfig", func(t *testing.T) {
		// Invalid app ID
		t.Setenv("APP_ID", "invalid-id")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "test-secret")

		flags := CommandLineFlags{}
		_, err := InitializeServerConfig(&ctx, flags)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid app id")

		// Missing app key
		t.Setenv("APP_ID", "12345")
		t.Setenv("APP_KEY", "")
		t.Setenv("APP_SECRET", "test-secret")

		_, err = InitializeServerConfig(&ctx, flags)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing app key/secret")
	})

	t.Run("StorageUsingRedis", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("STORAGE_MANAGER", "redis")
		t.Setenv("REDIS_URL", "redis://"+mr.Addr())

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "redis", config.StorageDriver)

		assert.NotNil(t, config.RedisInstance)
	})

	t.Run("PubSubUsingRedis", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("PUBSUB_MANAGER", "redis")
		t.Setenv("REDIS_URL", "redis://"+mr.Addr())

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "redis", config.PubSubDriver)

		assert.NotNil(t, config.RedisInstance)
	})

	t.Run("DispatcherUsingRedis", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("DISPATCHER_MANAGER", "redis")
		t.Setenv("REDIS_URL", "redis://"+mr.Addr())

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "redis", config.QueueDriver)

		assert.NotNil(t, config.RedisInstance)
	})

	t.Run("ChannelCacheUsingRedis", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("CHANNEL_CACHE_MANAGER", "redis")
		t.Setenv("REDIS_URL", "redis://"+mr.Addr())

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "redis", config.ChannelCacheDriver)

		assert.NotNil(t, config.RedisInstance)
	})

	t.Run("FailedRedisConnection", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("REDIS_URL", "redis://invalid-url:9999")
		t.Setenv("REDIS_USE_TLS", "true")
		t.Setenv("STORAGE_MANAGER", "redis")

		flags := CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		assert.Error(t, err)
		assert.Nil(t, config)
		// assert.Contains(t, err.Error(), "Error initializing Redis client:")
	})

}

func TestSetupApps(t *testing.T) {
	t.Run("ValidApp", func(t *testing.T) {
		setupTestEnv(t)

		config := &ServerConfig{}
		err := config.setupApps()
		assert.NoError(t, err)
		assert.Len(t, config.Apps, 1)

		app, ok := config.Apps["12345"]
		assert.True(t, ok)
		assert.Equal(t, "12345", app.AppID)
		assert.Equal(t, "test-key", app.AppKey)
		assert.Equal(t, "test-secret", app.AppSecret)
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		t.Setenv("APP_ID", "invalid-id")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "test-secret")

		config := &ServerConfig{}
		err := config.setupApps()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid app id")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		t.Setenv("APP_ID", "")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "test-secret")

		config := &ServerConfig{}
		err := config.setupApps()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid app id")
	})

	t.Run("MissingAppKey", func(t *testing.T) {
		t.Setenv("APP_ID", "12345")
		t.Setenv("APP_KEY", "")
		t.Setenv("APP_SECRET", "test-secret")

		config := &ServerConfig{}
		err := config.setupApps()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing app key/secret")
	})

	t.Run("MissingAppSecret", func(t *testing.T) {
		t.Setenv("APP_ID", "12345")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "")

		config := &ServerConfig{}
		err := config.setupApps()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing app key/secret")
	})
}

func TestLoadAppByID(t *testing.T) {
	config := &ServerConfig{
		Apps: map[AppID]AppConfig{
			"12345": {
				AppID:     "12345",
				AppKey:    "test-key",
				AppSecret: "test-secret",
			},
		},
	}

	t.Run("ExistingApp", func(t *testing.T) {
		app, err := config.LoadAppByID("12345")
		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "12345", app.AppID)
		assert.Equal(t, "test-key", app.AppKey)
		assert.Equal(t, "test-secret", app.AppSecret)
	})

	t.Run("NonExistingApp", func(t *testing.T) {
		app, err := config.LoadAppByID("99999")
		assert.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "app not found by id")
	})
}

func TestLoadAppByKey(t *testing.T) {
	config := &ServerConfig{
		Apps: map[AppID]AppConfig{
			"12345": {
				AppID:     "12345",
				AppKey:    "test-key",
				AppSecret: "test-secret",
			},
		},
	}

	t.Run("ExistingApp", func(t *testing.T) {
		app, err := config.LoadAppByKey("test-key")
		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "12345", app.AppID)
		assert.Equal(t, "test-key", app.AppKey)
		assert.Equal(t, "test-secret", app.AppSecret)
	})

	t.Run("NonExistingApp", func(t *testing.T) {
		app, err := config.LoadAppByKey("wrong-key")
		assert.Error(t, err)
		assert.Nil(t, app)
		assert.Contains(t, err.Error(), "app not found by key")
	})
}

func TestInitializeWebhookManager(t *testing.T) {
	t.Run("HttpWebhook", func(t *testing.T) {
		config := &ServerConfig{
			WebhookDriver: "http",
			WebhookURL:    "https://example.com/webhook",
		}

		err := config.InitializeWebhookManager()
		assert.NoError(t, err)
		assert.NotNil(t, config.WebhookManager)
	})

	t.Run("SnsWebhook", func(t *testing.T) {
		config := &ServerConfig{
			WebhookDriver:      "sns",
			WebhookSnsTopicArn: "arn:aws:sns:region:account:topic",
			WebhookSnsRegion:   "us-west-2",
		}

		err := config.InitializeWebhookManager()
		assert.NoError(t, err)
		assert.NotNil(t, config.WebhookManager)
	})

	t.Run("InvalidDriver", func(t *testing.T) {
		config := &ServerConfig{
			WebhookDriver: "invalid",
		}

		err := config.InitializeWebhookManager()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid webhook driver")
	})
}
