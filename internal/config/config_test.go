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
	"pusher/internal"
	"pusher/internal/constants"
	"pusher/internal/util"
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

	os.Unsetenv("CHANNEL_CACHE_MANAGER")
	os.Unsetenv("REDIS_PREFIX")
	os.Unsetenv("APP_ACTIVITY_TIMEOUT")
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

	assert.True(t, util.FileExists(tempFile.Name()), "Should return true for existing file")

	// Test with non-existing file
	assert.False(t, util.FileExists("/path/to/nonexistent/file"), "Should return false for non-existing file")
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
		flags := &CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		server, err := internal.NewServer(ctx, config, flags)
		assert.NoError(t, err)
		assert.NotNil(t, server)

		// Check default values
		assert.Equal(t, "6001", config.Port)
		assert.Equal(t, "0.0.0.0", config.BindAddress)
		assert.Equal(t, constants.PRODUCTION, config.Env)

		assert.Equal(t, "local", config.QueueDriver)

		assert.Equal(t, "local", config.ChannelCacheDriver)
		assert.Equal(t, "pusher", config.RedisPrefix)

		// Check app config
		assert.Len(t, len(server.AppManager.GetAllApps()), 1)
		app := server.AppManager.GetAllApps()[0]
		assert.Equal(t, "12345", app.ID)
		assert.Equal(t, "test-key", app.Key)
		assert.Equal(t, "test-secret", app.Secret)

		assert.Equal(t, 60*time.Second, app.ActivityTimeout)
		assert.Equal(t, int64(100), app.MaxPresenceMembersPerChannel)
		assert.Equal(t, 10*1024, app.MaxPresenceMemberSizeInKb)
	})

	t.Run("CustomEnvConfig", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("BIND_PORT", "8080")
		t.Setenv("BIND_ADDR", "127.0.0.1")
		t.Setenv("APP_ENV", "development")
		t.Setenv("APP_ACTIVITY_TIMEOUT", "120")
		t.Setenv("APP_MAX_PRESENCE_USERS", "200")
		t.Setenv("APP_MAX_PRESENCE_USER_DATA_KB", "20")

		flags := &CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		server, err := internal.NewServer(ctx, config, flags)
		assert.NoError(t, err)
		assert.NotNil(t, server)

		// Check custom values
		assert.Equal(t, "8080", config.Port)
		assert.Equal(t, "127.0.0.1", config.BindAddress)
		assert.Equal(t, "development", config.Env)

		assert.Len(t, len(server.AppManager.GetAllApps()), 1)
		app := server.AppManager.GetAllApps()[0]

		assert.Equal(t, 120*time.Second, app.ActivityTimeout)
		assert.Equal(t, int64(200), app.MaxPresenceMembersPerChannel)
		assert.Equal(t, 20*1024, app.MaxPresenceMemberSizeInKb)
	})

	t.Run("CommandLinePortOverride", func(t *testing.T) {
		setupTestEnv(t)

		// Set command line flag to override env
		flags := &CommandLineFlags{
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

		flags := &CommandLineFlags{}
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

	t.Run("ChannelCacheUsingRedis", func(t *testing.T) {
		setupTestEnv(t)

		t.Setenv("CHANNEL_CACHE_MANAGER", "redis")
		t.Setenv("REDIS_URL", "redis://"+mr.Addr())

		flags := &CommandLineFlags{
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

		flags := &CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		assert.Error(t, err)
		assert.Nil(t, config)
		// assert.Contains(t, err.Error(), "Error initializing Redis client:")
	})

}

func TestSetupApps(t *testing.T) {
	ctx := context.Background()
	t.Run("ValidApp", func(t *testing.T) {
		setupTestEnv(t)

		flags := &CommandLineFlags{
			Port: "",
		}

		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		server, err := internal.NewServer(ctx, config, flags)
		assert.NoError(t, err)
		assert.NotNil(t, server)

		assert.Len(t, server.AppManager.GetAllApps(), 1)

		app, err := server.AppManager.FindByID("12345")
		assert.NoError(t, err)
		assert.Equal(t, "12345", app.ID)
		assert.Equal(t, "test-key", app.Key)
		assert.Equal(t, "test-secret", app.Secret)
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		t.Setenv("APP_ID", "invalid-id")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "test-secret")
		flags := &CommandLineFlags{
			Port: "",
		}
		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		_, err = internal.NewServer(ctx, config, flags)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid app id")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		t.Setenv("APP_ID", "")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "test-secret")

		flags := &CommandLineFlags{
			Port: "",
		}
		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		_, err = internal.NewServer(ctx, config, flags)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid app id")
	})

	t.Run("MissingAppKey", func(t *testing.T) {
		t.Setenv("APP_ID", "12345")
		t.Setenv("APP_KEY", "")
		t.Setenv("APP_SECRET", "test-secret")

		flags := &CommandLineFlags{
			Port: "",
		}
		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		_, err = internal.NewServer(ctx, config, flags)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing app key/secret")
	})

	t.Run("MissingAppSecret", func(t *testing.T) {
		t.Setenv("APP_ID", "12345")
		t.Setenv("APP_KEY", "test-key")
		t.Setenv("APP_SECRET", "")

		flags := &CommandLineFlags{
			Port: "",
		}
		config, err := InitializeServerConfig(&ctx, flags)
		require.NoError(t, err)
		assert.NotNil(t, config)

		_, err = internal.NewServer(ctx, config, flags)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing app key/secret")
	})
}
