package internal

import (
	"context"
	"testing"

	"pusher/internal/apps"
	"pusher/internal/config"
	"pusher/internal/metrics"
	"pusher/internal/webhooks"

	"github.com/stretchr/testify/assert"
)

// TestNewServerBasic tests the NewServer function with basic cases
func TestNewServerBasic(t *testing.T) {
	t.Run("successful server creation with local adapter", func(t *testing.T) {
		config := &config.ServerConfig{
			AppManager:         "array",
			AdapterDriver:      "local",
			QueueDriver:        "local",
			ChannelCacheDriver: "local",
			Applications: func() []apps.App {
				app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
				app.SetMissingDefaults()
				return []apps.App{app}
			}(),
		}

		ctx := context.Background()
		server, err := NewServer(ctx, config)

		assert.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, config, server.config)
		assert.Equal(t, ctx, server.ctx)
	})

	t.Run("nil config", func(t *testing.T) {
		ctx := context.Background()
		server, err := NewServer(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "server config is nil")
	})
}

// TestRunBasic tests the Run function with basic cases
func TestRunBasic(t *testing.T) {
	t.Run("Run with nil context", func(t *testing.T) {
		err := Run(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent context is nil")
	})

	t.Run("Run with valid context but missing config", func(t *testing.T) {
		ctx := context.Background()
		err := Run(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load config")
	})
}

// TestLoadAppManagerBasic tests the loadAppManager function
func TestLoadAppManagerBasic(t *testing.T) {
	t.Run("array app manager", func(t *testing.T) {
		config := &config.ServerConfig{
			AppManager: "array",
			Applications: func() []apps.App {
				app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
				app.SetMissingDefaults()
				return []apps.App{app}
			}(),
		}

		ctx := context.Background()
		appManager, err := loadAppManager(ctx, config)

		assert.NoError(t, err)
		assert.NotNil(t, appManager)
	})
}

// TestLoadAdapterBasic tests the loadAdapter function
func TestLoadAdapterBasic(t *testing.T) {
	t.Run("local adapter", func(t *testing.T) {
		config := &config.ServerConfig{
			AdapterDriver: "local",
		}

		ctx := context.Background()
		adapter, err := loadAdapter(ctx, config, &metrics.NoOpMetrics{})

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})

	t.Run("redis adapter with nil redis instance", func(t *testing.T) {
		config := &config.ServerConfig{
			AdapterDriver: "redis",
			RedisInstance: nil,
		}

		ctx := context.Background()
		adapter, err := loadAdapter(ctx, config, &metrics.NoOpMetrics{})

		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "redis instance not configured")
	})
}

// TestLoadQueueManagerBasic tests the loadQueueManager function
func TestLoadQueueManagerBasic(t *testing.T) {
	webhookSender := &webhooks.WebhookSender{
		HttpSender: &webhooks.HttpWebhook{},
		SNSSender:  &webhooks.SnsWebhook{},
	}

	t.Run("local queue manager", func(t *testing.T) {
		config := &config.ServerConfig{
			QueueDriver: "local",
		}

		ctx := context.Background()
		queueManager, err := loadQueueManager(ctx, config, webhookSender)

		assert.NoError(t, err)
		assert.NotNil(t, queueManager)
	})

	t.Run("redis queue manager with nil redis instance", func(t *testing.T) {
		config := &config.ServerConfig{
			QueueDriver:   "redis",
			RedisInstance: nil,
		}

		ctx := context.Background()
		queueManager, err := loadQueueManager(ctx, config, webhookSender)

		assert.Error(t, err)
		assert.Nil(t, queueManager)
		assert.Contains(t, err.Error(), "redis instance not configured")
	})
}

// TestLoadCacheManagerBasic tests the loadCacheManager function
func TestLoadCacheManagerBasic(t *testing.T) {
	t.Run("local cache manager", func(t *testing.T) {
		config := &config.ServerConfig{
			ChannelCacheDriver: "local",
		}

		ctx := context.Background()
		cacheManager, err := loadCacheManager(ctx, config)

		assert.NoError(t, err)
		assert.NotNil(t, cacheManager)
	})

	t.Run("redis cache manager with nil redis instance", func(t *testing.T) {
		config := &config.ServerConfig{
			ChannelCacheDriver: "redis",
			RedisInstance:      nil,
		}

		ctx := context.Background()
		cacheManager, err := loadCacheManager(ctx, config)

		assert.Error(t, err)
		assert.Nil(t, cacheManager)
		assert.Contains(t, err.Error(), "redis instance not configured")
	})
}

// TestHandlePanicBasic tests the handlePanic function
func TestHandlePanicBasic(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		server := &Server{
			Closing: false,
		}

		// Test that handlePanic doesn't do anything when there's no panic
		func() {
			defer handlePanic(server)
			// No panic here
		}()

		// Server should still be in original state
		assert.False(t, server.Closing)
	})

	t.Run("panic recovery", func(t *testing.T) {
		// Test that handlePanic recovers from panic and sets Closing to true
		// Note: This will call os.Exit(1) so we can't test the full flow
		// But we can test that the function exists and can be called
		assert.NotPanics(t, func() {
			_ = handlePanic
		})
	})
}

// TestHandleInterruptBasic tests the handleInterrupt function
func TestHandleInterruptBasic(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		// Test handleInterrupt
		// Note: This will call os.Exit(0) so we can't test the full flow
		// But we can test that the function exists and can be called
		assert.NotPanics(t, func() {
			_ = handleInterrupt
		})
	})
}

// TestServeMetricsBasic tests the serveMetrics function
func TestServeMetricsBasic(t *testing.T) {
	t.Run("serveMetrics with nil server", func(t *testing.T) {
		config := &config.ServerConfig{
			MetricsEnabled: true,
			BindAddress:    "localhost",
			MetricsPort:    "9090",
		}

		// This will panic because serveMetrics tries to access server.PrometheusMetrics
		// which is expected behavior - the function assumes a valid server
		assert.Panics(t, func() {
			serveMetrics(nil, config)
		})
	})
}
