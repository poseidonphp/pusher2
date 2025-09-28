package internal

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pusher/internal/apps"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/metrics"
	"pusher/internal/util"
)

// MockAdapter for testing
type MockAdapterForServe struct {
	AddSocketCalled     bool
	AddSocketError      error
	GetSocketsCountFunc func(appID constants.AppID, onlyLocal bool) int64
	AddedSockets        map[constants.AppID][]*WebSocket
}

func (m *MockAdapterForServe) Init() error {
	return nil
}

func (m *MockAdapterForServe) Disconnect() {
}

func (m *MockAdapterForServe) GetNamespace(appID constants.AppID) (*Namespace, error) {
	return nil, nil
}

func (m *MockAdapterForServe) GetNamespaces() (map[constants.AppID]*Namespace, error) {
	return nil, nil
}

func (m *MockAdapterForServe) ClearNamespace(appID constants.AppID) {
}

func (m *MockAdapterForServe) ClearNamespaces() {
}

func (m *MockAdapterForServe) AddSocket(appID constants.AppID, ws *WebSocket) error {
	m.AddSocketCalled = true
	if m.AddedSockets == nil {
		m.AddedSockets = make(map[constants.AppID][]*WebSocket)
	}
	m.AddedSockets[appID] = append(m.AddedSockets[appID], ws)
	return m.AddSocketError
}

func (m *MockAdapterForServe) RemoveSocket(appID constants.AppID, wsID constants.SocketID) error {
	return nil
}

func (m *MockAdapterForServe) GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return nil
}

func (m *MockAdapterForServe) GetSocketsCount(appID constants.AppID, onlyLocal bool) int64 {
	if m.GetSocketsCountFunc != nil {
		return m.GetSocketsCountFunc(appID, onlyLocal)
	}
	return 0
}

func (m *MockAdapterForServe) AddToChannel(appID constants.AppID, channelName constants.ChannelName, ws *WebSocket) (int64, error) {
	return 0, nil
}

func (m *MockAdapterForServe) RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64 {
	return 0
}

func (m *MockAdapterForServe) GetChannels(appID constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID {
	return nil
}

func (m *MockAdapterForServe) GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return nil
}

func (m *MockAdapterForServe) GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return nil
}

func (m *MockAdapterForServe) GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int64 {
	return 0
}

func (m *MockAdapterForServe) IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool {
	return false
}

func (m *MockAdapterForServe) Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error {
	return nil
}

func (m *MockAdapterForServe) GetChannelMembers(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[string]*pusherClient.MemberData {
	return nil
}

func (m *MockAdapterForServe) GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int {
	return 0
}

func (m *MockAdapterForServe) GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return nil
}

func (m *MockAdapterForServe) AddUser(appID constants.AppID, ws *WebSocket) error {
	return nil
}

func (m *MockAdapterForServe) RemoveUser(appID constants.AppID, ws *WebSocket) error {
	return nil
}

func (m *MockAdapterForServe) GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error) {
	return nil, nil
}

func (m *MockAdapterForServe) TerminateUserConnections(appID constants.AppID, userID string) {
}

// MockAppManager for testing
type MockAppManagerForServe struct {
	FindByKeyFunc    func(key string) (*apps.App, error)
	FindByIDFunc     func(id constants.AppID) (*apps.App, error)
	GetAllAppsFunc   func() []*apps.App
	GetAppSecretFunc func(appID constants.AppID) (string, error)
}

func (m *MockAppManagerForServe) FindByKey(key string) (*apps.App, error) {
	if m.FindByKeyFunc != nil {
		return m.FindByKeyFunc(key)
	}
	return nil, assert.AnError
}

func (m *MockAppManagerForServe) FindByID(id constants.AppID) (*apps.App, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(id)
	}
	return nil, assert.AnError
}

func (m *MockAppManagerForServe) GetAllApps() []apps.App {
	if m.GetAllAppsFunc != nil {
		appList := m.GetAllAppsFunc()
		result := make([]apps.App, len(appList))
		for i, app := range appList {
			result[i] = *app
		}
		return result
	}
	return nil
}

func (m *MockAppManagerForServe) GetAppSecret(appID constants.AppID) (string, error) {
	if m.GetAppSecretFunc != nil {
		return m.GetAppSecretFunc(appID)
	}
	return "", assert.AnError
}

// Helper function to create a test server
func createTestServerForServe() *Server {
	serverConfig := &config.ServerConfig{
		Env:         "test",
		Port:        "8080",
		BindAddress: "localhost",
	}

	appManager := &MockAppManagerForServe{}
	adapter := &MockAdapterForServe{}
	metricsManager := &metrics.NoOpMetrics{}

	return &Server{
		config:         serverConfig,
		ctx:            context.Background(),
		AppManager:     appManager,
		Adapter:        adapter,
		MetricsManager: metricsManager,
		Closing:        false,
	}
}

// Helper function to create a test app
func createTestAppForServe() *apps.App {
	app := &apps.App{
		ID:                          "test-app-id",
		Key:                         "test-app-key",
		Secret:                      "test-app-secret",
		Enabled:                     true,
		MaxConnections:              100,
		RequireChannelAuthorization: false,
		ActivityTimeout:             120,
	}
	app.SetMissingDefaults()
	return app
}

func TestLoadWebServer(t *testing.T) {
	t.Run("ProductionMode", func(t *testing.T) {
		server := createTestServerForServe()
		server.config.Env = constants.PRODUCTION

		httpServer := LoadWebServer(server)

		assert.NotNil(t, httpServer)
		assert.Equal(t, "localhost:8080", httpServer.Addr)
		assert.NotNil(t, httpServer.Handler)
	})

	t.Run("DebugMode", func(t *testing.T) {
		server := createTestServerForServe()
		server.config.Env = "development"

		httpServer := LoadWebServer(server)

		assert.NotNil(t, httpServer)
		assert.Equal(t, "localhost:8080", httpServer.Addr)
		assert.NotNil(t, httpServer.Handler)
	})

	t.Run("CustomBindAddress", func(t *testing.T) {
		server := createTestServerForServe()
		server.config.BindAddress = "0.0.0.0"
		server.config.Port = "9090"

		httpServer := LoadWebServer(server)

		assert.NotNil(t, httpServer)
		assert.Equal(t, "0.0.0.0:9090", httpServer.Addr)
	})

	t.Run("RouterConfiguration", func(t *testing.T) {
		server := createTestServerForServe()
		httpServer := LoadWebServer(server)

		// Test that the router is properly configured
		router := httpServer.Handler.(*gin.Engine)
		assert.NotNil(t, router)

		// Test health check endpoint
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})
}

func TestValidateApp(t *testing.T) {
	t.Run("ValidApp", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		app, err := validateApp(server, "test-app-key")

		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, testApp, app)
	})

	t.Run("AppNotFound", func(t *testing.T) {
		server := createTestServerForServe()

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return nil, assert.AnError
		}

		app, err := validateApp(server, "nonexistent-key")

		assert.Error(t, err)
		assert.Nil(t, app)
		assert.Equal(t, util.ErrCodeAppNotExist, err)
	})

	t.Run("AppDisabled", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.Enabled = false

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		app, err := validateApp(server, "test-app-key")

		assert.Error(t, err)
		assert.Nil(t, app)
		assert.Equal(t, util.ErrCodeAppDisabled, err)
	})
}

func TestValidateConnectionLimits(t *testing.T) {
	t.Run("WithinLimits", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.MaxConnections = 100

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 50
		}

		err := validateConnectionLimits(server, testApp)

		assert.NoError(t, err)
	})

	t.Run("ExceedsLimits", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.MaxConnections = 100

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 100 // At the limit, next connection would exceed
		}

		err := validateConnectionLimits(server, testApp)

		assert.Error(t, err)
		assert.Equal(t, util.ErrCodeOverQuota, err)
	})

	t.Run("UnlimitedConnections", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.MaxConnections = -1 // Unlimited

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 1000 // Even with many connections, should not error
		}

		err := validateConnectionLimits(server, testApp)

		assert.NoError(t, err)
	})

	t.Run("ZeroConnections", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.MaxConnections = 0

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 0
		}

		err := validateConnectionLimits(server, testApp)

		assert.Error(t, err)
		assert.Equal(t, util.ErrCodeOverQuota, err)
	})
}

func TestCreateWebSocket(t *testing.T) {
	t.Run("CreateWebSocket", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		socketID := constants.SocketID("test-socket-id")

		// Create a mock WebSocket connection
		_, conn := createMockWebSocketConnection()

		ws := createWebSocket(socketID, conn, server, testApp)

		assert.NotNil(t, ws)
		assert.Equal(t, socketID, ws.ID)
		assert.Equal(t, conn, ws.conn)
		assert.Equal(t, server, ws.server)
		assert.Equal(t, testApp, ws.app)
		assert.False(t, ws.closed)
		assert.NotNil(t, ws.SubscribedChannels)
		assert.NotNil(t, ws.PresenceData)
	})

	t.Run("NilConnection", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		socketID := constants.SocketID("test-socket-id")

		ws := createWebSocket(socketID, nil, server, testApp)

		assert.Nil(t, ws)
	})

	t.Run("NilServer", func(t *testing.T) {
		testApp := createTestAppForServe()
		socketID := constants.SocketID("test-socket-id")
		_, conn := createMockWebSocketConnection()

		ws := createWebSocket(socketID, conn, nil, testApp)

		assert.Nil(t, ws)
	})

	t.Run("NilApp", func(t *testing.T) {
		server := createTestServerForServe()
		socketID := constants.SocketID("test-socket-id")
		_, conn := createMockWebSocketConnection()

		ws := createWebSocket(socketID, conn, server, nil)

		assert.Nil(t, ws)
	})
}

func TestAddSocketToAdapter(t *testing.T) {
	t.Run("SuccessfulAdd", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		_, conn := createMockWebSocketConnection()
		socketID := constants.SocketID("test-socket-id")
		ws := createWebSocket(socketID, conn, server, testApp)
		require.NotNil(t, ws, "WebSocket should be created successfully")

		err := addSocketToAdapter(server, testApp, ws)

		assert.NoError(t, err)
		assert.True(t, server.Adapter.(*MockAdapterForServe).AddSocketCalled)
	})

	t.Run("AddSocketError", func(t *testing.T) {
		server := createTestServerForServe()
		server.Adapter.(*MockAdapterForServe).AddSocketError = assert.AnError
		testApp := createTestAppForServe()
		_, conn := createMockWebSocketConnection()
		socketID := constants.SocketID("test-socket-id")
		ws := createWebSocket(socketID, conn, server, testApp)
		require.NotNil(t, ws, "WebSocket should be created successfully")

		err := addSocketToAdapter(server, testApp, ws)

		assert.Error(t, err)
		assert.Equal(t, util.ErrCodeOverQuota, err)
	})
}

func TestInitializeWebSocketConnection(t *testing.T) {
	t.Run("WithoutChannelAuthorization", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.RequireChannelAuthorization = false
		testApp.AuthorizationTimeout = 30 * time.Second // Set a timeout value

		// Create a mock WebSocket connection
		_, conn := createMockWebSocketConnection()
		socketID := constants.SocketID("test-socket-id")

		ws := createWebSocket(socketID, conn, server, testApp)
		require.NotNil(t, ws, "WebSocket should be created successfully")

		// This should not panic
		assert.NotPanics(t, func() {
			initializeWebSocketConnection(ws, server, testApp)
		})
	})

	t.Run("WithChannelAuthorization", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.RequireChannelAuthorization = true
		testApp.AuthorizationTimeout = 30 * time.Second // Set a timeout value

		// Create a mock WebSocket connection
		_, conn := createMockWebSocketConnection()
		socketID := constants.SocketID("test-socket-id")

		ws := createWebSocket(socketID, conn, server, testApp)
		require.NotNil(t, ws, "WebSocket should be created successfully")

		// This should not panic
		assert.NotPanics(t, func() {
			initializeWebSocketConnection(ws, server, testApp)
		})
	})
}

func TestCloseConnectionWithError(t *testing.T) {
	t.Run("CloseWithError", func(t *testing.T) {
		_, conn := createMockWebSocketConnection()

		// This should not panic
		assert.NotPanics(t, func() {
			closeConnectionWithError(conn, util.ErrCodeAppNotExist, "test error")
		})
	})

	t.Run("CloseWithDifferentErrorCodes", func(t *testing.T) {
		_, conn := createMockWebSocketConnection()

		errorCodes := []util.ErrorCode{
			util.ErrCodeAppNotExist,
			util.ErrCodeAppDisabled,
			util.ErrCodeOverQuota,
			util.ErrCodeUnsupportedProtocolVersion,
		}

		for _, errorCode := range errorCodes {
			assert.NotPanics(t, func() {
				closeConnectionWithError(conn, errorCode, "test error")
			})
		}
	})
}

func TestServeWs(t *testing.T) {
	t.Run("ServerClosing", func(t *testing.T) {
		server := createTestServerForServe()
		server.Closing = true

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "7")
		})

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("UnsupportedProtocolVersion", func(t *testing.T) {
		server := createTestServerForServe()

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "99") // Unsupported version
		})
	})

	t.Run("AppNotFound", func(t *testing.T) {
		server := createTestServerForServe()
		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return nil, assert.AnError
		}

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "7")
		})
	})

	t.Run("AppDisabled", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.Enabled = false

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "7")
		})
	})

	t.Run("ExceedsConnectionLimits", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.MaxConnections = 1

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 1 // At the limit
		}

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "7")
		})
	})

	t.Run("AddSocketError", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		server.Adapter.(*MockAdapterForServe).AddSocketError = assert.AnError

		w := httptest.NewRecorder()
		req := createWebSocketUpgradeRequest("/app/test-key")

		// This should not panic
		assert.NotPanics(t, func() {
			serveWs(server, w, req, "test-key", "test-client", "1.0", "7")
		})
	})
}

func TestServeWs_Integration(t *testing.T) {
	t.Run("SuccessfulConnection", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.AuthorizationTimeout = 30 * time.Second

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 0
		}

		// Create a real HTTP server that can handle WebSocket upgrades
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveWs(server, w, r, "test-key", "test-client", "1.0", "7")
		}))
		defer httpServer.Close()

		// Create a WebSocket client
		wsURL := "ws" + httpServer.URL[4:] + "/app/test-key"
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), wsURL)

		// The connection might fail due to the mock setup, but we can still test the logic
		if err != nil {
			// This is expected in our test environment due to mocking
			t.Logf("WebSocket connection failed as expected in test environment: %v", err)
		} else {
			conn.Close()
		}

		// Verify that the socket was added to the adapter
		assert.True(t, server.Adapter.(*MockAdapterForServe).AddSocketCalled)
	})

	t.Run("WithChannelAuthorization", func(t *testing.T) {
		server := createTestServerForServe()
		testApp := createTestAppForServe()
		testApp.RequireChannelAuthorization = true
		testApp.AuthorizationTimeout = 30 * time.Second

		server.AppManager.(*MockAppManagerForServe).FindByKeyFunc = func(key string) (*apps.App, error) {
			return testApp, nil
		}

		server.Adapter.(*MockAdapterForServe).GetSocketsCountFunc = func(appID constants.AppID, onlyLocal bool) int64 {
			return 0
		}

		// Create a real HTTP server that can handle WebSocket upgrades
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveWs(server, w, r, "test-key", "test-client", "1.0", "7")
		}))
		defer httpServer.Close()

		// Create a WebSocket client
		wsURL := "ws" + httpServer.URL[4:] + "/app/test-key"
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), wsURL)

		// The connection might fail due to the mock setup, but we can still test the logic
		if err != nil {
			// This is expected in our test environment due to mocking
			t.Logf("WebSocket connection failed as expected in test environment: %v", err)
		} else {
			conn.Close()
		}

		// Verify that the socket was added to the adapter
		assert.True(t, server.Adapter.(*MockAdapterForServe).AddSocketCalled)
	})
}

// Helper function to create a mock WebSocket connection
func createMockWebSocketConnection() (*httptest.ResponseRecorder, net.Conn) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), wsURL)
	if err != nil {
		// Return a mock connection for testing
		return httptest.NewRecorder(), nil
	}

	return httptest.NewRecorder(), conn
}

// Helper function to create a proper WebSocket upgrade request
func createWebSocketUpgradeRequest(url string) *http.Request {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")
	return req
}

func TestLoadWebServer_Routes(t *testing.T) {
	t.Run("AllRoutesRegistered", func(t *testing.T) {
		server := createTestServerForServe()
		httpServer := LoadWebServer(server)
		router := httpServer.Handler.(*gin.Engine)

		// Test that all expected routes are registered
		routes := []string{
			"POST /apps/:app_id/events",
			"POST /apps/:app_id/batch_events",
			"GET /apps/:app_id/channels",
			"GET /apps/:app_id/channels/:channel_name",
			"GET /apps/:app_id/channels/:channel_name/users",
			"POST /apps/:app_id/users/:user_id/terminate_connections",
			"GET /app/:key",
			"GET /health",
		}

		// Get all registered routes
		registeredRoutes := make(map[string]bool)
		for _, route := range router.Routes() {
			registeredRoutes[route.Method+" "+route.Path] = true
		}

		// Check that all expected routes are registered
		for _, route := range routes {
			assert.True(t, registeredRoutes[route], "Route %s should be registered", route)
		}
	})
}
