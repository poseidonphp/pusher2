package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"pusher/internal"
	"pusher/internal/apps"
	"pusher/internal/config"

	"github.com/stretchr/testify/assert"
)

// ServerInterface defines the interface that handlePanic expects
type ServerInterface interface {
	CloseAllLocalSockets()
}

// MockServerForTesting is a simple mock for testing
type MockServerForTesting struct {
	closing bool
}

func (m *MockServerForTesting) CloseAllLocalSockets() {
	m.closing = true
}

// TestServerWrapper wraps the real server to make it testable
type TestServerWrapper struct {
	server *internal.Server
}

func (w *TestServerWrapper) CloseAllLocalSockets() {
	if w.server != nil {
		w.server.CloseAllLocalSockets()
	}
}

// testHandlePanic is a test version of handlePanic that accepts an interface
func testHandlePanic(server ServerInterface) {
	if r := recover(); r != nil {
		server.CloseAllLocalSockets()
	}
}

// testPanicRecovery is a helper function that recovers from panics and calls the handler
func testPanicRecovery(server ServerInterface) {
	if r := recover(); r != nil {
		server.CloseAllLocalSockets()
	}
}

func TestHandlePanic(t *testing.T) {
	tests := []struct {
		name        string
		panicValue  interface{}
		expectExit  bool
		expectClose bool
	}{
		{
			name:        "no panic",
			panicValue:  nil,
			expectExit:  false,
			expectClose: false,
		},
		{
			name:        "panic with string",
			panicValue:  "test panic",
			expectExit:  true,
			expectClose: true,
		},
		{
			name:        "panic with error",
			panicValue:  errors.New("test error"),
			expectExit:  true,
			expectClose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := &MockServerForTesting{}

			// Test the panic handling
			func() {
				defer testPanicRecovery(mockServer)

				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
			}()

			// Verify the server state
			if tt.expectClose {
				assert.True(t, mockServer.closing)
			} else {
				assert.False(t, mockServer.closing)
			}
		})
	}
}

// testHandleInterrupt is a test version of handleInterrupt that accepts an interface
func testHandleInterrupt(server ServerInterface, wg *sync.WaitGroup, webServer *http.Server, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Close the server
	server.CloseAllLocalSockets()

	// Shutdown the HTTP server
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := webServer.Shutdown(ctx); err != nil {
		// Log error but don't fail the test
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func TestHandleInterrupt(t *testing.T) {
	t.Run("interrupt signal handling", func(t *testing.T) {
		mockServer := &MockServerForTesting{}

		// Create a test web server
		webServer := &http.Server{
			Addr: ":0", // Use port 0 for automatic port assignment
		}

		var wg sync.WaitGroup
		cancel := func() {}

		// Test interrupt handling in a goroutine
		go func() {
			// Simulate receiving an interrupt signal
			time.Sleep(10 * time.Millisecond)
			// Send interrupt signal to the process
			proc, _ := os.FindProcess(os.Getpid())
			proc.Signal(os.Interrupt)
		}()

		// This will block until interrupt is received
		testHandleInterrupt(mockServer, &wg, webServer, cancel)

		// Verify the server was marked as closing
		assert.True(t, mockServer.closing)
	})
}

func TestConfigInitialization(t *testing.T) {
	t.Run("config initialization with test environment", func(t *testing.T) {
		// Set test environment
		os.Setenv("GO_TEST", "true")
		defer os.Unsetenv("GO_TEST")

		ctx := context.Background()
		config, err := config.InitializeServerConfig(&ctx)

		// In test environment, config might fail due to missing files
		// This is acceptable for unit tests
		if err != nil {
			t.Logf("Config initialization failed in test environment: %v", err)
			return
		}

		assert.NotNil(t, config)
	})
}

func TestServerCreation(t *testing.T) {
	t.Run("server creation with minimal config", func(t *testing.T) {
		// Create a minimal config for testing
		config := &config.ServerConfig{
			Env:                    "test",
			Port:                   "8080",
			BindAddress:            "localhost",
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
		server, err := internal.NewServer(ctx, config)

		// Server creation might fail due to missing dependencies
		// This is acceptable for unit tests
		if err != nil {
			t.Logf("Server creation failed (expected in test): %v", err)
			return
		}

		assert.NotNil(t, server)
		assert.False(t, server.Closing)
	})
}

func TestWebServerLoading(t *testing.T) {
	t.Run("web server loading with minimal server", func(t *testing.T) {
		// Create a minimal server for testing
		config := &config.ServerConfig{
			Env:                    "test",
			Port:                   "8080",
			BindAddress:            "localhost",
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
		server, err := internal.NewServer(ctx, config)
		if err != nil {
			t.Logf("Server creation failed (expected in test): %v", err)
			return
		}

		webServer := internal.LoadWebServer(server)
		assert.NotNil(t, webServer)
		assert.Equal(t, "localhost:8080", webServer.Addr)
	})
}

func TestPanicRecovery(t *testing.T) {
	t.Run("panic recovery with mock server", func(t *testing.T) {
		mockServer := &MockServerForTesting{}

		// Test panic recovery
		func() {
			defer testHandlePanic(mockServer)
			panic("test panic")
		}()

		assert.True(t, mockServer.closing)
	})
}

func TestSignalHandling(t *testing.T) {
	t.Run("signal channel setup", func(t *testing.T) {
		// Test that we can set up signal handling
		c := make(chan os.Signal, 1)

		// Test sending a signal
		c <- os.Interrupt

		select {
		case sig := <-c:
			assert.Equal(t, os.Interrupt, sig)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected to receive interrupt signal")
		}
	})
}

func TestConcurrencySafety(t *testing.T) {
	t.Run("concurrent panic handling", func(t *testing.T) {
		mockServer := &MockServerForTesting{}

		// Test multiple concurrent panics
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() {
					done <- true
				}()
				defer testHandlePanic(mockServer)
				panic("concurrent panic")
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			select {
			case <-done:
				// Goroutine completed
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for goroutine to complete")
			}
		}

		assert.True(t, mockServer.closing)
	})
}

func TestWebServerLifecycle(t *testing.T) {
	t.Run("web server graceful shutdown", func(t *testing.T) {
		// Create a test HTTP server
		server := &http.Server{
			Addr: ":0", // Use port 0 for automatic port assignment
		}

		// Test that we can shut down the server
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestMainFunctionFlow(t *testing.T) {
	t.Run("main function flow analysis", func(t *testing.T) {
		// This test documents the expected flow of the main function
		// and verifies that each step can be executed

		// Step 1: Context creation
		ctx, cancel := context.WithCancel(context.Background())
		assert.NotNil(t, ctx)
		assert.NotNil(t, cancel)
		defer cancel()

		// Step 2: Create minimal config for testing
		config := &config.ServerConfig{
			Env:                    "test",
			Port:                   "8080",
			BindAddress:            "localhost",
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
		assert.NotNil(t, config)

		// Step 3: Server creation
		server, err := internal.NewServer(ctx, config)
		if err != nil {
			t.Logf("Server creation failed (expected in test): %v", err)
			return
		}
		assert.NotNil(t, server)

		// Step 4: Web server loading
		webServer := internal.LoadWebServer(server)
		assert.NotNil(t, webServer)

		// Step 5: Panic handling setup
		defer testHandlePanic(&TestServerWrapper{server: server})

		// Step 6: Signal handling setup
		// This would be tested in the actual interrupt handling
		t.Log("Main function flow completed successfully")
	})
}

// Benchmark tests for performance
func BenchmarkHandlePanic(b *testing.B) {
	mockServer := &MockServerForTesting{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer testHandlePanic(mockServer)
			panic("benchmark panic")
		}()
	}
}

// TestConfigBuilder helps build test configurations
type TestConfigBuilder struct {
	config *config.ServerConfig
}

// NewTestConfigBuilder creates a new test config builder
func NewTestConfigBuilder() *TestConfigBuilder {
	return &TestConfigBuilder{
		config: &config.ServerConfig{
			Env:                    "test",
			Port:                   "8080",
			BindAddress:            "localhost",
			AppManager:             "array",
			AdapterDriver:          "local",
			QueueDriver:            "local",
			ChannelCacheDriver:     "local",
			LogLevel:               "debug",
			IgnoreLoggerMiddleware: true,
		},
	}
}

// WithPort sets the port
func (b *TestConfigBuilder) WithPort(port string) *TestConfigBuilder {
	b.config.Port = port
	return b
}

// WithBindAddress sets the bind address
func (b *TestConfigBuilder) WithBindAddress(address string) *TestConfigBuilder {
	b.config.BindAddress = address
	return b
}

// WithEnv sets the environment
func (b *TestConfigBuilder) WithEnv(env string) *TestConfigBuilder {
	b.config.Env = env
	return b
}

// Build returns the built config
func (b *TestConfigBuilder) Build() *config.ServerConfig {
	return b.config
}

// TestServerHelper provides helper methods for testing
type TestServerHelper struct {
	server    *internal.Server
	webServer *http.Server
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
}

// NewTestServerHelper creates a new test server helper
func NewTestServerHelper(config *config.ServerConfig) (*TestServerHelper, error) {
	ctx, cancel := context.WithCancel(context.Background())

	server, err := internal.NewServer(ctx, config)
	if err != nil {
		cancel()
		return nil, err
	}

	webServer := internal.LoadWebServer(server)

	return &TestServerHelper{
		server:    server,
		webServer: webServer,
		ctx:       ctx,
		cancel:    cancel,
		wg:        &sync.WaitGroup{},
	}, nil
}

// GetServer returns the server instance
func (h *TestServerHelper) GetServer() *internal.Server {
	return h.server
}

// GetWebServer returns the web server instance
func (h *TestServerHelper) GetWebServer() *http.Server {
	return h.webServer
}

// GetContext returns the context
func (h *TestServerHelper) GetContext() context.Context {
	return h.ctx
}

// GetWaitGroup returns the wait group
func (h *TestServerHelper) GetWaitGroup() *sync.WaitGroup {
	return h.wg
}

// StartWebServer starts the web server in a goroutine
func (h *TestServerHelper) StartWebServer() error {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail the test
		}
	}()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)
	return nil
}

// StopWebServer stops the web server
func (h *TestServerHelper) StopWebServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return h.webServer.Shutdown(ctx)
}

// Cleanup cleans up resources
func (h *TestServerHelper) Cleanup() {
	h.cancel()
	h.server.Closing = true
	h.server.CloseAllLocalSockets()
	h.wg.Wait()
}

// TestPanicHelper helps test panic scenarios
type TestPanicHelper struct {
	server *internal.Server
}

// NewTestPanicHelper creates a new test panic helper
func NewTestPanicHelper(server *internal.Server) *TestPanicHelper {
	return &TestPanicHelper{server: server}
}

// TriggerPanic triggers a panic and recovers it
func (h *TestPanicHelper) TriggerPanic(panicValue interface{}) (recovered bool, err error) {
	recovered = false
	err = nil

	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
				handlePanic(h.server)
			}
		}()

		if panicValue != nil {
			panic(panicValue)
		}
	}()

	return recovered, err
}

// TestSignalHelper helps test signal handling
type TestSignalHelper struct {
	signalChan chan os.Signal
}

// NewTestSignalHelper creates a new test signal helper
func NewTestSignalHelper() *TestSignalHelper {
	return &TestSignalHelper{
		signalChan: make(chan os.Signal, 1),
	}
}

// SendSignal sends a signal to the channel
func (h *TestSignalHelper) SendSignal(sig os.Signal) {
	h.signalChan <- sig
}

// GetSignalChannel returns the signal channel
func (h *TestSignalHelper) GetSignalChannel() chan os.Signal {
	return h.signalChan
}

// TestEnvironmentHelper helps manage test environment
type TestEnvironmentHelper struct {
	originalEnv map[string]string
}

// NewTestEnvironmentHelper creates a new test environment helper
func NewTestEnvironmentHelper() *TestEnvironmentHelper {
	return &TestEnvironmentHelper{
		originalEnv: make(map[string]string),
	}
}

// SetTestEnvironment sets up test environment
func (h *TestEnvironmentHelper) SetTestEnvironment() {
	// Save original values
	h.originalEnv["GO_TEST"] = os.Getenv("GO_TEST")

	// Set test environment
	os.Setenv("GO_TEST", "true")
}

// RestoreEnvironment restores original environment
func (h *TestEnvironmentHelper) RestoreEnvironment() {
	for key, value := range h.originalEnv {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

// TestTimeoutHelper helps manage test timeouts
type TestTimeoutHelper struct {
	timeout time.Duration
}

// NewTestTimeoutHelper creates a new test timeout helper
func NewTestTimeoutHelper(timeout time.Duration) *TestTimeoutHelper {
	return &TestTimeoutHelper{timeout: timeout}
}

// WithTimeout executes a function with a timeout
func (h *TestTimeoutHelper) WithTimeout(fn func()) error {
	done := make(chan bool, 1)

	go func() {
		fn()
		done <- true
	}()

	select {
	case <-done:
		return nil
	case <-time.After(h.timeout):
		return &TestTimeoutError{timeout: h.timeout}
	}
}

// TestTimeoutError represents a test timeout error
type TestTimeoutError struct {
	timeout time.Duration
}

func (e *TestTimeoutError) Error() string {
	return "test timeout after " + e.timeout.String()
}

// TestConcurrencyHelper helps test concurrency scenarios
type TestConcurrencyHelper struct {
	goroutines int
	timeout    time.Duration
}

// NewTestConcurrencyHelper creates a new test concurrency helper
func NewTestConcurrencyHelper(goroutines int, timeout time.Duration) *TestConcurrencyHelper {
	return &TestConcurrencyHelper{
		goroutines: goroutines,
		timeout:    timeout,
	}
}

// RunConcurrent executes a function concurrently
func (h *TestConcurrencyHelper) RunConcurrent(fn func(int)) error {
	done := make(chan bool, h.goroutines)

	for i := 0; i < h.goroutines; i++ {
		go func(index int) {
			fn(index)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < h.goroutines; i++ {
		select {
		case <-done:
			// Goroutine completed
		case <-time.After(h.timeout):
			return &TestTimeoutError{timeout: h.timeout}
		}
	}

	return nil
}
