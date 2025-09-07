package internal

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/util"
)

// MockAppManager for testing
type MockAppManager struct {
	apps map[string]*apps.App
}

func NewMockAppManager() *MockAppManager {
	return &MockAppManager{
		apps: make(map[string]*apps.App),
	}
}

func (m *MockAppManager) FindByID(appID constants.AppID) (*apps.App, error) {
	if app, exists := m.apps[string(appID)]; exists {
		return app, nil
	}
	return nil, fmt.Errorf("app not found: %s", appID)
}

func (m *MockAppManager) FindByKey(key string) (*apps.App, error) {
	for _, app := range m.apps {
		if app.Key == key {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app not found with key: %s", key)
}

func (m *MockAppManager) GetAllApps() []apps.App {
	var allApps []apps.App
	for _, app := range m.apps {
		allApps = append(allApps, *app)
	}
	return allApps
}

func (m *MockAppManager) GetAppSecret(id constants.AppID) (string, error) {
	if app, exists := m.apps[string(id)]; exists {
		return app.Secret, nil
	}
	return "", fmt.Errorf("app not found: %s", id)
}

func (m *MockAppManager) AddApp(app *apps.App) {
	m.apps[app.ID] = app
}

func (m *MockAppManager) GetApps() map[string]*apps.App {
	return m.apps
}

// Helper function to create a test app
func createTestAppForMiddleware() *apps.App {
	app := &apps.App{
		ID:      "test-app",
		Key:     "test-key",
		Secret:  "test-secret",
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test server
func createTestServerForMiddleware() (*Server, *MockAppManager) {
	appManager := NewMockAppManager()
	app := createTestAppForMiddleware()
	appManager.AddApp(app)

	server := &Server{
		AppManager: appManager,
	}

	return server, appManager
}

// Helper function to create a test request with signature
func createTestRequestWithSignature(method, url, body, secret string) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(body))

	// Add required query parameters for signature verification
	query := req.URL.Query()
	query.Set("auth_version", "1.0")
	query.Set("auth_timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.URL.RawQuery = query.Encode()

	// Generate signature using the same logic as the Verify function
	queryString := prepareQueryStringForTest(query)
	stringToSign := strings.Join([]string{strings.ToUpper(method), req.URL.Path, queryString}, "\n")
	signature := util.HmacSignature(stringToSign, secret)

	// Add signature to query parameters
	query.Set("auth_signature", signature)
	req.URL.RawQuery = query.Encode()

	return req
}

// Helper function to prepare query string for signature generation (copied from util/signature.go)
func prepareQueryStringForTest(params url.Values) string {
	var keys []string
	for key := range params {
		keys = append(keys, strings.ToLower(key))
	}

	sort.Strings(keys)
	var pieces []string

	for _, key := range keys {
		pieces = append(pieces, key+"="+params.Get(key))
	}

	return strings.Join(pieces, "&")
}

func TestGetAppFromContext(t *testing.T) {
	t.Run("AppExists", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		app := createTestAppForMiddleware()
		c.Set("app", app)

		retrievedApp, exists := getAppFromContext(c)

		assert.True(t, exists)
		assert.Equal(t, app, retrievedApp)
	})

	t.Run("AppNotExists", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		retrievedApp, exists := getAppFromContext(c)

		assert.False(t, exists)
		assert.Nil(t, retrievedApp)
	})

	t.Run("AppWrongType", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("app", "not-an-app")

		retrievedApp, exists := getAppFromContext(c)

		assert.False(t, exists)
		assert.Nil(t, retrievedApp)
	})
}

func TestAppMiddleware(t *testing.T) {
	t.Run("ValidAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.GET("/apps/:app_id/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/apps/test-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.GET("/apps/:app_id/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/apps/nonexistent-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "app not found")
	})

	t.Run("AppInContext", func(t *testing.T) {
		server, appManager := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.GET("/apps/:app_id/test", func(c *gin.Context) {
			// Check if app was set in context
			appFromContext, exists := c.Get("app")
			assert.True(t, exists)

			// Get the expected app from the app manager
			expectedApp, err := appManager.FindByID("test-app")
			assert.NoError(t, err)
			assert.Equal(t, expectedApp, appFromContext)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/apps/test-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSignatureMiddleware(t *testing.T) {
	t.Run("ValidSignature", func(t *testing.T) {
		server, appManager := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.Use(SignatureMiddleware(server))
		router.POST("/apps/:app_id/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Get the app secret from the app manager
		app, err := appManager.FindByID("test-app")
		assert.NoError(t, err)

		body := `{"name": "test-event", "channel": "test-channel", "data": "test-data"}`
		req := createTestRequestWithSignature("POST", "/apps/test-app/events", body, app.Secret)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.Use(SignatureMiddleware(server))
		router.POST("/apps/:app_id/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		body := `{"name": "test-event", "channel": "test-channel", "data": "test-data"}`
		req := createTestRequestWithSignature("POST", "/apps/test-app/events", body, "wrong-secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(SignatureMiddleware(server))
		router.POST("/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("POST", "/events", strings.NewReader("{}"))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "App not found in context")
	})

	t.Run("MissingSignature", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.Use(SignatureMiddleware(server))
		router.POST("/apps/:app_id/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("POST", "/apps/test-app/events", strings.NewReader("{}"))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})
}

func TestCorsMiddleware(t *testing.T) {
	t.Run("CorsMiddlewareExecutes", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(CorsMiddleware(server))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("CorsMiddlewareContinues", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		executed := false

		router := gin.New()
		router.Use(CorsMiddleware(server))
		router.GET("/test", func(c *gin.Context) {
			executed = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, executed)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRateLimiterMiddleware(t *testing.T) {
	t.Run("RateLimiterMiddlewareExecutes", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(RateLimiterMiddleware(server))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("RateLimiterMiddlewareContinues", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		executed := false

		router := gin.New()
		router.Use(RateLimiterMiddleware(server))
		router.GET("/test", func(c *gin.Context) {
			executed = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, executed)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestLoggerMiddleware(t *testing.T) {
	t.Run("SuccessfulRequest", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Set("Referer", "https://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "GET")
		assert.Contains(t, logContent, "/test")
		assert.Contains(t, logContent, "200")
		assert.Contains(t, logContent, "test-agent")
		assert.Contains(t, logContent, "https://example.com")
	})

	t.Run("ErrorRequest", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Check that error log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "500")
	})

	t.Run("ClientErrorRequest", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Check that warning log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "400")
	})

	t.Run("GinErrors", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			c.Error(fmt.Errorf("test error"))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Check that error log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "test error")
	})

	t.Run("LatencyCalculation", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that latency was logged
		logContent := logOutput.String()
		assert.Contains(t, logContent, "latency")
	})

	t.Run("NegativeDataLength", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			// Set a negative size to test the handling
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.WriteString("test")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that dataLength was handled correctly
		logContent := logOutput.String()
		assert.Contains(t, logContent, "dataLength")
	})

	t.Run("ClientIPExtraction", func(t *testing.T) {
		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that client IP was logged
		logContent := logOutput.String()
		assert.Contains(t, logContent, "192.168.1.1")
	})
}

func TestMiddlewareIntegration(t *testing.T) {
	t.Run("FullMiddlewareChain", func(t *testing.T) {
		server, appManager := createTestServerForMiddleware()

		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.Use(CorsMiddleware(server))
		router.Use(RateLimiterMiddleware(server))
		router.Use(AppMiddleware(server))
		router.Use(SignatureMiddleware(server))
		router.POST("/apps/:app_id/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Get the app secret from the app manager
		app, err := appManager.FindByID("test-app")
		assert.NoError(t, err)

		body := `{"name": "test-event", "channel": "test-channel", "data": "test-data"}`
		req := createTestRequestWithSignature("POST", "/apps/test-app/events", body, app.Secret)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")

		// Check that all middlewares executed
		logContent := logOutput.String()
		assert.Contains(t, logContent, "POST")
		assert.Contains(t, logContent, "/apps/test-app/events")
		assert.Contains(t, logContent, "200")
	})

	t.Run("MiddlewareChainWithError", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		logger := logrus.New()
		var logOutput bytes.Buffer
		logger.SetOutput(&logOutput)
		logger.SetLevel(logrus.InfoLevel)

		router := gin.New()
		router.Use(LoggerMiddleware(logger))
		router.Use(CorsMiddleware(server))
		router.Use(RateLimiterMiddleware(server))
		router.Use(AppMiddleware(server))
		router.Use(SignatureMiddleware(server))
		router.POST("/apps/:app_id/events", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("POST", "/apps/nonexistent-app/events", strings.NewReader("{}"))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "app not found")

		// Check that error was logged
		logContent := logOutput.String()
		assert.Contains(t, logContent, "404")
	})
}

func TestTimeFormat(t *testing.T) {
	t.Run("TimeFormatConstant", func(t *testing.T) {
		// Test that the time format constant is correct
		testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
		formatted := testTime.Format(timeFormat)

		// Should match the expected format
		assert.Contains(t, formatted, "2023-12-25")
		assert.Contains(t, formatted, "15:30:45")
		assert.Contains(t, formatted, "+0000")
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("NilServerInMiddleware", func(t *testing.T) {
		// Test that middlewares handle nil server gracefully
		router := gin.New()
		router.Use(AppMiddleware(nil))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// This should panic due to nil pointer dereference
		assert.Panics(t, func() {
			router.ServeHTTP(w, req)
		})
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.GET("/apps/:app_id/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/apps//test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("VeryLongAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		longAppID := strings.Repeat("a", 1000)

		router := gin.New()
		router.Use(AppMiddleware(server))
		router.GET("/apps/:app_id/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/apps/"+longAppID+"/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
