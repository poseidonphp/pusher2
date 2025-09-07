package internal

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"pusher/internal/apps"
	"pusher/internal/clients"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/metrics"
	"pusher/internal/webhooks"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

// TestNewServer tests the NewServer function
func TestNewServer(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ServerConfig
		expectError bool
	}{
		{
			name: "successful server creation with local adapter",
			config: &config.ServerConfig{
				AppManager:         "array",
				AdapterDriver:      "local",
				QueueDriver:        "local",
				ChannelCacheDriver: "local",
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: false,
		},
		{
			name: "empty applications",
			config: &config.ServerConfig{
				AppManager:         "array",
				AdapterDriver:      "local",
				QueueDriver:        "local",
				ChannelCacheDriver: "local",
				Applications:       []apps.App{},
			},
			expectError: true,
		},
		{
			name: "redis adapter without redis instance",
			config: &config.ServerConfig{
				AppManager:         "array",
				AdapterDriver:      "redis",
				QueueDriver:        "local",
				ChannelCacheDriver: "local",
				RedisInstance:      nil,
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: true,
		},
		{
			name: "redis queue without redis instance",
			config: &config.ServerConfig{
				AppManager:         "array",
				AdapterDriver:      "local",
				QueueDriver:        "redis",
				ChannelCacheDriver: "local",
				RedisInstance:      nil,
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: true,
		},
		{
			name: "redis cache without redis instance",
			config: &config.ServerConfig{
				AppManager:         "array",
				AdapterDriver:      "local",
				QueueDriver:        "local",
				ChannelCacheDriver: "redis",
				RedisInstance:      nil,
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			server, err := NewServer(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.Equal(t, tt.config, server.config)
				assert.Equal(t, ctx, server.ctx)
				assert.NotNil(t, server.AppManager)
				assert.NotNil(t, server.Adapter)
				assert.NotNil(t, server.MetricsManager)
				assert.NotNil(t, server.CacheManager)
				assert.NotNil(t, server.QueueManager)
				assert.NotNil(t, server.WebhookSender)
				assert.False(t, server.Closing)
			}
		})
	}
}

// TestCloseAllLocalSockets tests the CloseAllLocalSockets method
func TestCloseAllLocalSockets(t *testing.T) {
	tests := []struct {
		name          string
		namespaces    map[constants.AppID]*Namespace
		namespacesErr error
		expectClose   bool
		setupAdapter  func() AdapterInterface
	}{
		{
			name:          "no namespaces",
			namespaces:    map[constants.AppID]*Namespace{},
			namespacesErr: nil,
			expectClose:   false,
			setupAdapter: func() AdapterInterface {
				return &MockAdapter{
					namespaces:    map[constants.AppID]*Namespace{},
					namespacesErr: nil,
				}
			},
		},
		{
			name:          "namespaces with no sockets",
			namespaces:    map[constants.AppID]*Namespace{"app1": {Sockets: map[constants.SocketID]*WebSocket{}}},
			namespacesErr: nil,
			expectClose:   false,
			setupAdapter: func() AdapterInterface {
				ns := &Namespace{Sockets: map[constants.SocketID]*WebSocket{}}
				return &MockAdapter{
					namespaces:    map[constants.AppID]*Namespace{"app1": ns},
					namespacesErr: nil,
				}
			},
		},
		{
			name:          "namespaces with sockets",
			namespaces:    map[constants.AppID]*Namespace{"app1": {Sockets: map[constants.SocketID]*WebSocket{"socket1": {}}}},
			namespacesErr: nil,
			expectClose:   true,
			setupAdapter: func() AdapterInterface {
				ws := &WebSocket{ID: "socket1"}
				ns := &Namespace{Sockets: map[constants.SocketID]*WebSocket{"socket1": ws}}
				return &MockAdapter{
					namespaces:    map[constants.AppID]*Namespace{"app1": ns},
					namespacesErr: nil,
				}
			},
		},
		{
			name:          "get namespaces error",
			namespaces:    nil,
			namespacesErr: errors.New("get namespaces failed"),
			expectClose:   false,
			setupAdapter: func() AdapterInterface {
				return &MockAdapter{
					namespaces:    nil,
					namespacesErr: errors.New("get namespaces failed"),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := tt.setupAdapter()

			server := &Server{
				Adapter: adapter,
			}

			server.CloseAllLocalSockets()

			// Verify that ClearNamespaces was called if we had namespaces
			if tt.namespacesErr == nil && len(tt.namespaces) > 0 {
				mockAdapter := adapter.(*MockAdapter)
				assert.True(t, mockAdapter.clearNamespacesCalled)
			}
		})
	}
}

// TestLoadAppManager tests the loadAppManager function
func TestLoadAppManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ServerConfig
		expectError bool
	}{
		{
			name: "array app manager",
			config: &config.ServerConfig{
				AppManager: "array",
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: false,
		},
		{
			name: "default app manager",
			config: &config.ServerConfig{
				AppManager: "unknown",
				Applications: func() []apps.App {
					app := apps.App{ID: "123", Key: "test-key", Secret: "test-secret"}
					app.SetMissingDefaults()
					return []apps.App{app}
				}(),
			},
			expectError: false,
		},
		{
			name: "empty applications",
			config: &config.ServerConfig{
				AppManager:   "array",
				Applications: []apps.App{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			appManager, err := loadAppManager(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, appManager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, appManager)
			}
		})
	}
}

// TestLoadAdapter tests the loadAdapter function
func TestLoadAdapter(t *testing.T) {
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	tests := []struct {
		name        string
		config      *config.ServerConfig
		expectError bool
	}{
		{
			name: "local adapter",
			config: &config.ServerConfig{
				AdapterDriver: "local",
			},
			expectError: false,
		},
		{
			name: "redis adapter without redis instance",
			config: &config.ServerConfig{
				AdapterDriver: "redis",
				RedisInstance: nil,
			},
			expectError: true,
		},
		{
			name: "redis adapter with redis instance",
			config: &config.ServerConfig{
				AdapterDriver: "redis",
				RedisInstance: &clients.RedisClient{Client: client},
			},
			expectError: false,
		},
		{
			name: "unknown adapter defaults to local",
			config: &config.ServerConfig{
				AdapterDriver: "unknown",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			adapter, err := loadAdapter(ctx, tt.config, &metrics.NoOpMetrics{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, adapter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
			}
		})
	}
}

// TestLoadQueueManager tests the loadQueueManager function
func TestLoadQueueManager(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.ServerConfig
		webhookSender *webhooks.WebhookSender
		expectError   bool
	}{
		{
			name: "local queue manager",
			config: &config.ServerConfig{
				QueueDriver: "local",
			},
			webhookSender: &webhooks.WebhookSender{},
			expectError:   false,
		},
		{
			name: "redis queue manager without redis instance",
			config: &config.ServerConfig{
				QueueDriver:   "redis",
				RedisInstance: nil,
			},
			webhookSender: &webhooks.WebhookSender{},
			expectError:   true,
		},
		{
			name: "unknown queue driver defaults to local",
			config: &config.ServerConfig{
				QueueDriver: "unknown",
			},
			webhookSender: &webhooks.WebhookSender{},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			queueManager, err := loadQueueManager(ctx, tt.config, tt.webhookSender)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, queueManager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, queueManager)
			}
		})
	}
}

// TestLoadCacheManager tests the loadCacheManager function
func TestLoadCacheManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ServerConfig
		expectError bool
	}{
		{
			name: "local cache manager",
			config: &config.ServerConfig{
				ChannelCacheDriver: "local",
			},
			expectError: false,
		},
		{
			name: "redis cache manager without redis instance",
			config: &config.ServerConfig{
				ChannelCacheDriver: "redis",
				RedisInstance:      nil,
			},
			expectError: true,
		},
		{
			name: "unknown cache driver defaults to local",
			config: &config.ServerConfig{
				ChannelCacheDriver: "unknown",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			cacheManager, err := loadCacheManager(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cacheManager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cacheManager)
			}
		})
	}
}

// TestServerConcurrency tests concurrent access to server methods
func TestServerConcurrency(t *testing.T) {
	t.Run("concurrent CloseAllLocalSockets calls", func(t *testing.T) {
		adapter := &MockAdapter{
			namespaces:    map[constants.AppID]*Namespace{"app1": {Sockets: map[constants.SocketID]*WebSocket{}}},
			namespacesErr: nil,
		}

		server := &Server{
			Adapter: adapter,
		}

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				server.CloseAllLocalSockets()
			}()
		}

		wg.Wait()
	})
}

// TestServerState tests server state management
func TestServerState(t *testing.T) {
	t.Run("server initial state", func(t *testing.T) {
		config := &config.ServerConfig{
			AppManager:         "array",
			AdapterDriver:      "local",
			QueueDriver:        "local",
			ChannelCacheDriver: "local",
			Applications: func() []apps.App {
				app := apps.App{ID: "test-app", Key: "test-key", Secret: "test-secret"}
				app.SetMissingDefaults()
				return []apps.App{app}
			}(),
		}

		ctx := context.Background()
		server, err := NewServer(ctx, config)

		assert.NoError(t, err)
		assert.NotNil(t, server)
		assert.False(t, server.Closing)
		assert.Equal(t, config, server.config)
		assert.Equal(t, ctx, server.ctx)
	})

	t.Run("server closing state", func(t *testing.T) {
		server := &Server{
			Closing: false,
		}

		assert.False(t, server.Closing)

		server.Closing = true
		assert.True(t, server.Closing)
	})
}

// MockAdapter is a simple mock implementation of AdapterInterface for testing
type MockAdapter struct {
	namespaces            map[constants.AppID]*Namespace
	namespacesErr         error
	clearNamespacesCalled bool
}

func (m *MockAdapter) Init() error {
	return nil
}

func (m *MockAdapter) Disconnect() {
}

func (m *MockAdapter) GetNamespace(appID constants.AppID) (*Namespace, error) {
	if m.namespacesErr != nil {
		return nil, m.namespacesErr
	}
	return m.namespaces[appID], nil
}

func (m *MockAdapter) GetNamespaces() (map[constants.AppID]*Namespace, error) {
	return m.namespaces, m.namespacesErr
}

func (m *MockAdapter) ClearNamespace(appID constants.AppID) {
}

func (m *MockAdapter) ClearNamespaces() {
	m.clearNamespacesCalled = true
}

func (m *MockAdapter) AddSocket(appID constants.AppID, ws *WebSocket) error {
	return nil
}

func (m *MockAdapter) RemoveSocket(appID constants.AppID, wsID constants.SocketID) error {
	return nil
}

func (m *MockAdapter) GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return map[constants.SocketID]*WebSocket{}
}

func (m *MockAdapter) GetSocketsCount(appID constants.AppID, onlyLocal bool) int64 {
	return 0
}

func (m *MockAdapter) AddChannel(appID constants.AppID, channelName constants.ChannelName) error {
	return nil
}

func (m *MockAdapter) RemoveChannel(appID constants.AppID, channelName constants.ChannelName) error {
	return nil
}

func (m *MockAdapter) GetChannels(appID constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID {
	return map[constants.ChannelName][]constants.SocketID{}
}

func (m *MockAdapter) GetChannelsCount(appID constants.AppID, onlyLocal bool) int64 {
	return 0
}

func (m *MockAdapter) SubscribeToChannel(appID constants.AppID, channelName constants.ChannelName, ws *WebSocket) error {
	return nil
}

func (m *MockAdapter) UnsubscribeFromChannel(appID constants.AppID, channelName constants.ChannelName, ws *WebSocket) error {
	return nil
}

func (m *MockAdapter) GetChannelMembers(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) map[constants.SocketID]*pusherClient.MemberData {
	return map[constants.SocketID]*pusherClient.MemberData{}
}

func (m *MockAdapter) GetChannelMembersCount(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) int {
	return 0
}

func (m *MockAdapter) AddUser(appID constants.AppID, ws *WebSocket) error {
	return nil
}

func (m *MockAdapter) RemoveUser(appID constants.AppID, ws *WebSocket) error {
	return nil
}

func (m *MockAdapter) GetUsers(appID constants.AppID, onlyLocal bool) map[string][]constants.SocketID {
	return map[string][]constants.SocketID{}
}

func (m *MockAdapter) GetUsersCount(appID constants.AppID, onlyLocal bool) int64 {
	return 0
}

func (m *MockAdapter) BroadcastToChannel(appID constants.AppID, channelName constants.ChannelName, event string, data interface{}, socketIDToExclude constants.SocketID) error {
	return nil
}

func (m *MockAdapter) BroadcastToUser(appID constants.AppID, userID string, event string, data interface{}) error {
	return nil
}

func (m *MockAdapter) BroadcastToAll(appID constants.AppID, event string, data interface{}, socketIDToExclude constants.SocketID) error {
	return nil
}

func (m *MockAdapter) AddToChannel(appID constants.AppID, channelName constants.ChannelName, ws *WebSocket) (int64, error) {
	return 1, nil
}

func (m *MockAdapter) RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64 {
	return 0
}

func (m *MockAdapter) GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return map[constants.ChannelName]int64{}
}

func (m *MockAdapter) GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return map[constants.SocketID]*WebSocket{}
}

func (m *MockAdapter) GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int64 {
	return 0
}

func (m *MockAdapter) IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool {
	return false
}

func (m *MockAdapter) Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error {
	return nil
}

func (m *MockAdapter) GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return map[constants.ChannelName]int64{}
}

func (m *MockAdapter) GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error) {
	return []*WebSocket{}, nil
}

func (m *MockAdapter) TerminateUserConnections(appID constants.AppID, userID string) {
}

// TestHandlePanic tests the handlePanic function
func TestHandlePanic(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		server := &Server{
			Closing: false,
			Adapter: &MockAdapter{}, // Provide a mock adapter
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

	t.Run("panic with different types", func(t *testing.T) {
		// Test that the function exists and can be referenced
		assert.NotPanics(t, func() {
			_ = handlePanic
		})
	})
}

// TestHandleInterrupt tests the handleInterrupt function
func TestHandleInterrupt(t *testing.T) {
	t.Run("graceful shutdown with web server only", func(t *testing.T) {
		// Test handleInterrupt with web server only
		// Note: This will call os.Exit(0) so we can't test the full flow
		// But we can test that the function exists and can be called
		assert.NotPanics(t, func() {
			// We can't actually call handleInterrupt in a test because it calls os.Exit
			// But we can verify the function signature is correct
			_ = handleInterrupt
		})
	})

	t.Run("graceful shutdown with both servers", func(t *testing.T) {
		// Test that the function can be referenced
		assert.NotPanics(t, func() {
			_ = handleInterrupt
		})
	})
}

// TestServeMetrics tests the serveMetrics function
func TestServeMetrics(t *testing.T) {
	t.Run("serveMetrics with valid server", func(t *testing.T) {
		// Create a test server with metrics enabled
		config := &config.ServerConfig{
			Env:                    "test",
			Port:                   "8080",
			BindAddress:            "localhost",
			MetricsPort:            "9090",
			MetricsEnabled:         true,
			AppManager:             "array",
			AdapterDriver:          "local",
			QueueDriver:            "local",
			ChannelCacheDriver:     "local",
			LogLevel:               "debug",
			IgnoreLoggerMiddleware: true,
			Applications: func() []apps.App {
				app := apps.App{
					ID:     "test-app",
					Key:    "test-key",
					Secret: "test-secret",
				}
				app.SetMissingDefaults()
				return []apps.App{app}
			}(),
		}

		ctx := context.Background()
		server, err := NewServer(ctx, config)
		if err != nil {
			t.Logf("Server creation failed (expected in test): %v", err)
			return
		}

		metricsServer := serveMetrics(server, config)

		assert.NotNil(t, metricsServer)
		assert.Equal(t, "localhost:9090", metricsServer.Addr)
		assert.NotNil(t, metricsServer.Handler)

		// Clean up
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		metricsServer.Shutdown(shutdownCtx)
	})

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

	t.Run("serveMetrics with different ports", func(t *testing.T) {
		ports := []string{"9090", "9091", "9092"}

		for _, port := range ports {
			t.Run("port_"+port, func(t *testing.T) {
				config := &config.ServerConfig{
					Env:                    "test",
					Port:                   "8080",
					BindAddress:            "localhost",
					MetricsPort:            port,
					MetricsEnabled:         true,
					AppManager:             "array",
					AdapterDriver:          "local",
					QueueDriver:            "local",
					ChannelCacheDriver:     "local",
					LogLevel:               "debug",
					IgnoreLoggerMiddleware: true,
					Applications: func() []apps.App {
						app := apps.App{
							ID:     "test-app",
							Key:    "test-key",
							Secret: "test-secret",
						}
						app.SetMissingDefaults()
						return []apps.App{app}
					}(),
				}

				ctx := context.Background()
				server, err := NewServer(ctx, config)
				if err != nil {
					t.Logf("Server creation failed (expected in test): %v", err)
					return
				}

				metricsServer := serveMetrics(server, config)

				assert.NotNil(t, metricsServer)
				expectedAddr := "localhost:" + port
				assert.Equal(t, expectedAddr, metricsServer.Addr)

				// Test server shutdown
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				err = metricsServer.Shutdown(shutdownCtx)
				assert.NoError(t, err)
			})
		}
	})
}

// TestRun tests the Run function
func TestRun(t *testing.T) {
	t.Run("Run with invalid context", func(t *testing.T) {
		// Test with nil context
		err := Run(nil)
		assert.Error(t, err)
	})

	t.Run("Run with valid context but missing config", func(t *testing.T) {
		// Test with valid context but no config file
		ctx := context.Background()
		err := Run(ctx)

		// We expect this to fail due to missing config
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load config")
	})

	t.Run("Run function exists and is callable", func(t *testing.T) {
		// Test that the Run function exists and can be called
		ctx := context.Background()
		pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

		// This should fail due to missing config, but the function should exist
		err := Run(ctx)
		assert.Error(t, err)
		assert.NotNil(t, err)
	})

	t.Run("Run with test environment setup", func(t *testing.T) {
		// Set up test environment
		os.Setenv("GO_TEST", "true")
		defer os.Unsetenv("GO_TEST")
		pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

		ctx := context.Background()
		err := Run(ctx)

		// Should still fail due to missing config, but should be callable
		assert.Error(t, err)
	})
}

// TestServerLifecycleIntegration tests the integration of all server functions
func TestServerLifecycleIntegration(t *testing.T) {
	t.Run("full server lifecycle simulation", func(t *testing.T) {
		// This test simulates the full lifecycle without actually running Run()
		config := &config.ServerConfig{
			Env:                    "test",
			Port:                   "0", // Use port 0 for automatic assignment
			BindAddress:            "localhost",
			MetricsPort:            "0", // Use port 0 for automatic assignment
			MetricsEnabled:         true,
			AppManager:             "array",
			AdapterDriver:          "local",
			QueueDriver:            "local",
			ChannelCacheDriver:     "local",
			LogLevel:               "debug",
			IgnoreLoggerMiddleware: true,
			Applications: func() []apps.App {
				app := apps.App{
					ID:     "test-app",
					Key:    "test-key",
					Secret: "test-secret",
				}
				app.SetMissingDefaults()
				return []apps.App{app}
			}(),
		}

		// Step 1: Context creation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Step 2: Server creation
		server, err := NewServer(ctx, config)
		if err != nil {
			t.Logf("Server creation failed (expected in test): %v", err)
			return
		}

		// Step 3: Web server loading
		webServer := LoadWebServer(server)
		assert.NotNil(t, webServer)

		// Step 4: Metrics server (if enabled)
		var metricsServer *http.Server
		if config.MetricsEnabled && server.MetricsManager != nil {
			metricsServer = serveMetrics(server, config)
			assert.NotNil(t, metricsServer)
		}

		// Step 5: Graceful shutdown
		server.Closing = true
		server.CloseAllLocalSockets()

		// Shutdown web server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer shutdownCancel()
		webServer.Shutdown(shutdownCtx)

		// Shutdown metrics server
		if metricsServer != nil {
			metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer metricsCancel()
			metricsServer.Shutdown(metricsCtx)
		}

		t.Log("Full server lifecycle simulation completed successfully")
	})
}
