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

	"github.com/stretchr/testify/assert"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/logger"
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

// newTestRouter serves handler at pattern behind the given middlewares.
func newTestRouter(pattern string, handler http.HandlerFunc, middlewares ...middleware) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(pattern, chain(handler, middlewares...))
	return mux
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
		app := createTestAppForMiddleware()
		req := withApp(httptest.NewRequest("GET", "/", nil), app)

		retrievedApp, exists := getAppFromContext(req)

		assert.True(t, exists)
		assert.Equal(t, app, retrievedApp)
	})

	t.Run("AppNotExists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		retrievedApp, exists := getAppFromContext(req)

		assert.False(t, exists)
		assert.Nil(t, retrievedApp)
	})

	t.Run("AppNil", func(t *testing.T) {
		req := withApp(httptest.NewRequest("GET", "/", nil), nil)

		retrievedApp, exists := getAppFromContext(req)

		assert.False(t, exists)
		assert.Nil(t, retrievedApp)
	})
}

func TestAppMiddleware(t *testing.T) {
	t.Run("ValidAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := newTestRouter("GET /apps/{app_id}/test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server))

		req := httptest.NewRequest("GET", "/apps/test-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := newTestRouter("GET /apps/{app_id}/test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server))

		req := httptest.NewRequest("GET", "/apps/nonexistent-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "app not found")
	})

	t.Run("AppInContext", func(t *testing.T) {
		server, appManager := createTestServerForMiddleware()

		router := newTestRouter("GET /apps/{app_id}/test", func(w http.ResponseWriter, r *http.Request) {
			// Check if app was set in context
			appFromContext, exists := getAppFromContext(r)
			assert.True(t, exists)

			// Get the expected app from the app manager
			expectedApp, err := appManager.FindByID("test-app")
			assert.NoError(t, err)
			assert.Equal(t, expectedApp, appFromContext)
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server))

		req := httptest.NewRequest("GET", "/apps/test-app/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSignatureMiddleware(t *testing.T) {
	t.Run("ValidSignature", func(t *testing.T) {
		server, appManager := createTestServerForMiddleware()

		router := newTestRouter("POST /apps/{app_id}/events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server), SignatureMiddleware(server))

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

		router := newTestRouter("POST /apps/{app_id}/events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server), SignatureMiddleware(server))

		body := `{"name": "test-event", "channel": "test-channel", "data": "test-data"}`
		req := createTestRequestWithSignature("POST", "/apps/test-app/events", body, "wrong-secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := newTestRouter("POST /events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, SignatureMiddleware(server))

		req := httptest.NewRequest("POST", "/events", strings.NewReader("{}"))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "App not found in context")
	})

	t.Run("MissingSignature", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := newTestRouter("POST /apps/{app_id}/events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server), SignatureMiddleware(server))

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

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, CorsMiddleware(server))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("CorsMiddlewareContinues", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		executed := false

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			executed = true
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, CorsMiddleware(server))

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

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, RateLimiterMiddleware(server))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("RateLimiterMiddlewareContinues", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		executed := false

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			executed = true
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, RateLimiterMiddleware(server))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, executed)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestLoggerMiddleware(t *testing.T) {
	t.Run("SuccessfulRequest", func(t *testing.T) {
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, LoggerMiddleware(logger))

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
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}, LoggerMiddleware(logger))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Check that error log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "500")
	})

	t.Run("ClientErrorRequest", func(t *testing.T) {
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		}, LoggerMiddleware(logger))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Check that warning log was written
		logContent := logOutput.String()
		assert.Contains(t, logContent, "400")
	})

	t.Run("LatencyCalculation", func(t *testing.T) {
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, LoggerMiddleware(logger))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that latency was logged
		logContent := logOutput.String()
		assert.Contains(t, logContent, "latency")
	})

	t.Run("DataLength", func(t *testing.T) {
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test"))
		}, LoggerMiddleware(logger))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that dataLength was handled correctly
		logContent := logOutput.String()
		assert.Contains(t, logContent, "dataLength")
	})

	t.Run("ClientIPExtraction", func(t *testing.T) {
		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, LoggerMiddleware(logger))

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

		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("POST /apps/{app_id}/events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, LoggerMiddleware(logger), CorsMiddleware(server), RateLimiterMiddleware(server), AppMiddleware(server), SignatureMiddleware(server))

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

		var logOutput bytes.Buffer
		logger := logger.NewAppLogger(logger.WithOutput(&logOutput))

		router := newTestRouter("POST /apps/{app_id}/events", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, LoggerMiddleware(logger), CorsMiddleware(server), RateLimiterMiddleware(server), AppMiddleware(server), SignatureMiddleware(server))

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

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("PanicReturns500", func(t *testing.T) {
		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}, RecoveryMiddleware)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AbortHandlerPropagates", func(t *testing.T) {
		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}, RecoveryMiddleware)

		assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/test", nil))
		})
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("HeadersAndBody", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeJSON(w, http.StatusCreated, map[string]string{"b": "2", "a": "1"})

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, `{"a":"1","b":"2"}`, w.Body.String())
	})

	t.Run("OmitEmptyKeepsLegacySemantics", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeJSON(w, http.StatusOK, struct {
			Count    int  `json:"count,omitempty"`
			Occupied bool `json:"occupied,omitempty"`
			Name     string
		}{Name: "x"})

		assert.Equal(t, `{"Name":"x"}`, w.Body.String())
	})

	t.Run("EmptyObject", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeEmptyJSON(w)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, `{}`, w.Body.String())
	})
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	assert.Equal(t, "10.0.0.1", clientIP(req))

	req.Header.Set("X-Real-IP", "10.0.0.2")
	assert.Equal(t, "10.0.0.2", clientIP(req))

	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.3")
	assert.Equal(t, "203.0.113.5", clientIP(req))
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
		router := newTestRouter("GET /test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(nil))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// This should panic due to nil pointer dereference
		assert.Panics(t, func() {
			router.ServeHTTP(w, req)
		})
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()

		router := newTestRouter("GET /apps/{app_id}/test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server))

		req := httptest.NewRequest("GET", "/apps//test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// ServeMux redirects the uncleaned path; the handler never runs.
		assert.NotEqual(t, http.StatusOK, w.Code)
	})

	t.Run("VeryLongAppID", func(t *testing.T) {
		server, _ := createTestServerForMiddleware()
		longAppID := strings.Repeat("a", 1000)

		router := newTestRouter("GET /apps/{app_id}/test", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
		}, AppMiddleware(server))

		req := httptest.NewRequest("GET", "/apps/"+longAppID+"/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
