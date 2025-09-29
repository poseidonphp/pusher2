package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMetricsHandler tests the constructor
func TestNewMetricsHandler(t *testing.T) {
	t.Run("ValidMetrics", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		assert.NotNil(t, handler)
		assert.Equal(t, metrics, handler.metrics)
	})

	t.Run("NilMetrics", func(t *testing.T) {
		// This should not panic, but the handler won't work properly
		handler := NewMetricsHandler(nil)

		assert.NotNil(t, handler)
		assert.Nil(t, handler.metrics)
	})
}

// TestPrometheusHandler tests the Prometheus metrics handler
func TestPrometheusHandler(t *testing.T) {
	t.Run("ValidHandler", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		promHandler := handler.PrometheusHandler()

		assert.NotNil(t, promHandler)

		// Test that the handler responds correctly
		req := httptest.NewRequest("GET", "/metrics", nil)
		rr := httptest.NewRecorder()

		promHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/plain")

		// Should contain our custom metrics
		body := rr.Body.String()
		assert.Contains(t, body, "test_metrics_connections_total")
		assert.Contains(t, body, "test_metrics_active_connections")
	})

	t.Run("NilMetrics", func(t *testing.T) {
		handler := NewMetricsHandler(nil)

		// This should panic because we're trying to access nil metrics
		assert.Panics(t, func() {
			handler.PrometheusHandler()
		})
	})
}

// TestJSONHandler tests the JSON metrics handler
func TestJSONHandler(t *testing.T) {
	t.Run("ValidHandler", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		jsonHandler := handler.JSONHandler()

		assert.NotNil(t, jsonHandler)

		// Test that the handler responds correctly
		req := httptest.NewRequest("GET", "/metrics/json", nil)
		rr := httptest.NewRecorder()

		jsonHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		// Should return valid JSON
		body := rr.Body.Bytes()
		var jsonData map[string]interface{}
		err := json.Unmarshal(body, &jsonData)
		require.NoError(t, err)

		// Should contain expected fields
		assert.Contains(t, jsonData, "timestamp")
		assert.Contains(t, jsonData, "namespace")
		assert.Contains(t, jsonData, "subsystem")

		assert.Equal(t, "test", jsonData["namespace"])
		assert.Equal(t, "metrics", jsonData["subsystem"])
	})
}

// TestTextHandler tests the plain text metrics handler
func TestTextHandler(t *testing.T) {
	t.Run("ValidHandler", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		textHandler := handler.TextHandler()

		assert.NotNil(t, textHandler)

		// Test that the handler responds correctly
		req := httptest.NewRequest("GET", "/metrics/text", nil)
		rr := httptest.NewRecorder()

		textHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))

		// Should contain our custom metrics in text format
		body := rr.Body.String()
		assert.Contains(t, body, "test_metrics_connections_total")
		assert.Contains(t, body, "test_metrics_active_connections")
		assert.Greater(t, len(body), 0)
	})
}

// TestSetupMetricsRoutes tests the route setup functionality
func TestSetupMetricsRoutes(t *testing.T) {
	t.Run("ValidSetup", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)
		mux := http.NewServeMux()

		handler.SetupMetricsRoutes(mux)

		// Test all three routes
		testCases := []struct {
			path     string
			expected string
		}{
			{"/metrics", "test_metrics_connections_total"},
			{"/metrics/json", `"namespace":"test"`},
			{"/metrics/text", "test_metrics_active_connections"},
		}

		for _, tc := range testCases {
			req := httptest.NewRequest("GET", tc.path, nil)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), tc.expected)
		}
	})

	t.Run("NilMetrics", func(t *testing.T) {
		handler := NewMetricsHandler(nil)
		mux := http.NewServeMux()

		// This should panic because we're trying to access nil metrics
		assert.Panics(t, func() {
			handler.SetupMetricsRoutes(mux)
		})
	})
}

// TestHTTPIntegration tests full HTTP server integration
func TestHTTPIntegration(t *testing.T) {
	t.Run("FullServer", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		// Create a test server
		mux := http.NewServeMux()
		handler.SetupMetricsRoutes(mux)

		server := httptest.NewServer(mux)
		defer server.Close()

		// Test all endpoints
		endpoints := []string{
			"/metrics",
			"/metrics/json",
			"/metrics/text",
		}

		for _, endpoint := range endpoints {
			t.Run("Endpoint_"+endpoint, func(t *testing.T) {
				resp, err := http.Get(server.URL + endpoint)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Verify content type based on endpoint
				switch endpoint {
				case "/metrics":
					assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
				case "/metrics/json":
					assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
				case "/metrics/text":
					assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
				}
			})
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		metrics := createTestPrometheusMetrics()
		handler := NewMetricsHandler(metrics)

		mux := http.NewServeMux()
		handler.SetupMetricsRoutes(mux)

		server := httptest.NewServer(mux)
		defer server.Close()

		// Make concurrent requests to test thread safety
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				resp, err := http.Get(server.URL + "/metrics")
				if err != nil {
					t.Errorf("Request failed: %v", err)
					return
				}
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestContentTypeHeaders tests that correct content types are set
func TestContentTypeHeaders(t *testing.T) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)

	testCases := []struct {
		name         string
		handler      http.Handler
		expectedType string
	}{
		{
			name:         "PrometheusHandler",
			handler:      handler.PrometheusHandler(),
			expectedType: "text/plain",
		},
		{
			name:         "JSONHandler",
			handler:      handler.JSONHandler(),
			expectedType: "application/json",
		},
		{
			name:         "TextHandler",
			handler:      handler.TextHandler(),
			expectedType: "text/plain",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			tc.handler.ServeHTTP(rr, req)

			contentType := rr.Header().Get("Content-Type")
			if tc.expectedType == "text/plain" {
				assert.Contains(t, contentType, tc.expectedType)
			} else {
				assert.Equal(t, tc.expectedType, contentType)
			}
		})
	}
}

// TestResponseStatusCodes tests that all handlers return 200 OK
func TestResponseStatusCodes(t *testing.T) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)

	handlers := map[string]http.Handler{
		"PrometheusHandler": handler.PrometheusHandler(),
		"JSONHandler":       handler.JSONHandler(),
		"TextHandler":       handler.TextHandler(),
	}

	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestNonGetRequests tests that handlers respond to different HTTP methods
func TestNonGetRequests(t *testing.T) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)

	methods := []string{"POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run("Method_"+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			handler.JSONHandler().ServeHTTP(rr, req)

			// All methods should return 200 OK (metrics are typically read-only)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestLargeResponse tests that handlers can handle large metric responses
func TestLargeResponse(t *testing.T) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)

	// Generate some metrics to make response larger
	for i := 0; i < 1000; i++ {
		metrics.MarkNewConnection("test-app")
		metrics.MarkWsMessageSent("test-app", "test-message")
	}

	testCases := []struct {
		name    string
		handler http.Handler
	}{
		{"PrometheusHandler", handler.PrometheusHandler()},
		// {"JSONHandler", handler.JSONHandler()},
		{"TextHandler", handler.TextHandler()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			tc.handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Greater(t, len(rr.Body.Bytes()), 1000) // Should have substantial content
		})
	}
}

// TestMetricsHandlerFields tests the internal structure
func TestMetricsHandlerFields(t *testing.T) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)

	// Test that the metrics field is set correctly
	assert.Equal(t, metrics, handler.metrics)
}

// Benchmark tests for performance
func BenchmarkPrometheusHandler(b *testing.B) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)
	promHandler := handler.PrometheusHandler()

	req := httptest.NewRequest("GET", "/metrics", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		promHandler.ServeHTTP(rr, req)
	}
}

func BenchmarkJSONHandler(b *testing.B) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)
	jsonHandler := handler.JSONHandler()

	req := httptest.NewRequest("GET", "/metrics/json", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		jsonHandler.ServeHTTP(rr, req)
	}
}

func BenchmarkTextHandler(b *testing.B) {
	metrics := createTestPrometheusMetrics()
	handler := NewMetricsHandler(metrics)
	textHandler := handler.TextHandler()

	req := httptest.NewRequest("GET", "/metrics/text", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		textHandler.ServeHTTP(rr, req)
	}
}
