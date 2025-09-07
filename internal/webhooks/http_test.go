package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, fmt.Errorf("mock client not configured")
}

// Helper function to create a test webhook
func createTestWebhook() pusherClient.Webhook {
	return pusherClient.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusherClient.WebhookEvent{
			{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test data"}`,
			},
		},
	}
}

// Helper function to create a test webhook with multiple events
func createTestWebhookWithMultipleEvents() pusherClient.Webhook {
	return pusherClient.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusherClient.WebhookEvent{
			{
				Name:    "channel_occupied",
				Channel: "test-channel-1",
				Data:    `{"message": "test data 1"}`,
			},
			{
				Name:    "channel_vacated",
				Channel: "test-channel-2",
				Data:    `{"message": "test data 2"}`,
			},
		},
	}
}

func TestHttpWebhook_Send(t *testing.T) {
	t.Run("SuccessfulWebhook", func(t *testing.T) {
		// Create a test server that responds with 200 OK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request method
			assert.Equal(t, "POST", r.Method)

			// Verify the content type
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify the Pusher headers
			assert.Equal(t, "test-key", r.Header.Get("X-Pusher-Key"))
			assert.NotEmpty(t, r.Header.Get("X-Pusher-Signature"))

			// Read and verify the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var webhook pusherClient.Webhook
			err = json.Unmarshal(body, &webhook)
			require.NoError(t, err)
			assert.NotEmpty(t, webhook.TimeMs)
			assert.Len(t, webhook.Events, 1)
			assert.Equal(t, "channel_occupied", webhook.Events[0].Name)

			// Respond with success
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("EmptyURL", func(t *testing.T) {
		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, "", "test-key", "test-secret")

		assert.NoError(t, err) // Should return nil for empty URL
	})

	t.Run("InvalidURL", func(t *testing.T) {
		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, "invalid-url", "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported protocol scheme")
	})

	t.Run("ServerError", func(t *testing.T) {
		// Create a test server that responds with 500 Internal Server Error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})

	t.Run("NotFoundError", func(t *testing.T) {
		// Create a test server that responds with 404 Not Found
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404 Not Found")
	})

	t.Run("BadRequestError", func(t *testing.T) {
		// Create a test server that responds with 400 Bad Request
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "400 Bad Request")
	})

	t.Run("UnauthorizedError", func(t *testing.T) {
		// Create a test server that responds with 401 Unauthorized
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401 Unauthorized")
	})

	t.Run("ForbiddenError", func(t *testing.T) {
		// Create a test server that responds with 403 Forbidden
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "403 Forbidden")
	})

	t.Run("MultipleEvents", func(t *testing.T) {
		// Create a test server that verifies multiple events
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var webhook pusherClient.Webhook
			err = json.Unmarshal(body, &webhook)
			require.NoError(t, err)
			assert.Len(t, webhook.Events, 2)
			assert.Equal(t, "channel_occupied", webhook.Events[0].Name)
			assert.Equal(t, "channel_vacated", webhook.Events[1].Name)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhookWithMultipleEvents()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("EmptyWebhook", func(t *testing.T) {
		// Create a test server that verifies empty webhook
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var webhook pusherClient.Webhook
			err = json.Unmarshal(body, &webhook)
			require.NoError(t, err)
			assert.Empty(t, webhook.Events)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{},
		}

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("LargeWebhook", func(t *testing.T) {
		// Create a webhook with large data
		largeData := make([]byte, 10000) // 10KB of data
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		webhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "large_event",
					Channel: "test-channel",
					Data:    string(largeData),
				},
			},
		}

		// Create a test server that verifies large data
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var receivedWebhook pusherClient.Webhook
			err = json.Unmarshal(body, &receivedWebhook)
			require.NoError(t, err)
			assert.Len(t, receivedWebhook.Events, 1)
			assert.Equal(t, "large_event", receivedWebhook.Events[0].Name)
			assert.Greater(t, len(receivedWebhook.Events[0].Data), 1000) // Just verify it's large

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})
}

func TestHttpWebhook_Send_EdgeCases(t *testing.T) {
	t.Run("NetworkTimeout", func(t *testing.T) {
		// Create a test server that takes a long time to respond
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Simulate slow response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		// This test might timeout depending on the HTTP client configuration
		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		// The test might pass or fail depending on timeout settings
		// We just verify that the method doesn't panic
		if err != nil {
			assert.Contains(t, err.Error(), "timeout")
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		// Create a webhook with data that might cause JSON marshaling issues
		webhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "test_event",
					Channel: "test-channel",
					Data:    "valid json data",
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err) // Valid JSON should not cause errors
	})

	t.Run("EmptyAppKey", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the Pusher headers
			assert.Equal(t, "", r.Header.Get("X-Pusher-Key"))
			assert.NotEmpty(t, r.Header.Get("X-Pusher-Signature"))

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("EmptyAppSecret", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the Pusher headers
			assert.Equal(t, "test-key", r.Header.Get("X-Pusher-Key"))
			// Empty secret should still generate a signature (HMAC with empty string)
			assert.NotEmpty(t, r.Header.Get("X-Pusher-Signature"))

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "")

		assert.NoError(t, err)
	})

	t.Run("SpecialCharactersInData", func(t *testing.T) {
		webhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "special_chars_event",
					Channel: "test-channel",
					Data:    `{"special": "chars: !@#$%^&*()_+-=[]{}|;':\",./<>?` + "`" + `"}`,
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and verify the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var receivedWebhook pusherClient.Webhook
			err = json.Unmarshal(body, &receivedWebhook)
			require.NoError(t, err)
			assert.Len(t, receivedWebhook.Events, 1)
			assert.Contains(t, receivedWebhook.Events[0].Data, "special")
			assert.Contains(t, receivedWebhook.Events[0].Data, "chars:")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})
}

func TestHttpWebhook_Send_Headers(t *testing.T) {
	t.Run("VerifyHeaders", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify all expected headers
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "test-key", r.Header.Get("X-Pusher-Key"))

			// Verify signature is present and not empty
			signature := r.Header.Get("X-Pusher-Signature")
			assert.NotEmpty(t, signature)

			// Verify signature format (should be HMAC signature)
			assert.Greater(t, len(signature), 10) // Basic length check

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("DifferentAppKeys", func(t *testing.T) {
		testCases := []struct {
			name      string
			appKey    string
			appSecret string
		}{
			{"SimpleKey", "simple-key", "simple-secret"},
			{"ComplexKey", "complex-key-123", "complex-secret-456"},
			{"LongKey", "very-long-app-key-with-many-characters", "very-long-app-secret-with-many-characters"},
			{"SpecialChars", "key-with-special-chars!@#", "secret-with-special-chars$%^"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tc.appKey, r.Header.Get("X-Pusher-Key"))

					// Verify signature is present
					signature := r.Header.Get("X-Pusher-Signature")
					assert.NotEmpty(t, signature)

					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				}))
				defer server.Close()

				httpWebhook := &HttpWebhook{}
				webhook := createTestWebhook()

				err := httpWebhook.Send(webhook, server.URL, tc.appKey, tc.appSecret)

				assert.NoError(t, err)
			})
		}
	})
}

func TestHttpWebhook_Send_ResponseHandling(t *testing.T) {
	t.Run("SuccessResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.NoError(t, err)
	})

	t.Run("CreatedResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("Created"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err) // Only 200 OK is considered success
		assert.Contains(t, err.Error(), "201 Created")
	})

	t.Run("AcceptedResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Accepted"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err) // Only 200 OK is considered success
		assert.Contains(t, err.Error(), "202 Accepted")
	})

	t.Run("NoContentResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}
		webhook := createTestWebhook()

		err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")

		assert.Error(t, err) // Only 200 OK is considered success
		assert.Contains(t, err.Error(), "204 No Content")
	})
}

func TestHttpWebhook_Send_Integration(t *testing.T) {
	t.Run("FullWorkflow", func(t *testing.T) {
		// Create a comprehensive test that verifies the entire workflow
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method
			assert.Equal(t, "POST", r.Method)

			// Verify headers
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "integration-test-key", r.Header.Get("X-Pusher-Key"))

			// Verify signature
			signature := r.Header.Get("X-Pusher-Signature")
			assert.NotEmpty(t, signature)

			// Read and verify request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var webhook pusherClient.Webhook
			err = json.Unmarshal(body, &webhook)
			require.NoError(t, err)

			// Verify webhook structure
			assert.Greater(t, webhook.TimeMs, 0)
			assert.Len(t, webhook.Events, 1)
			assert.Equal(t, "integration_test", webhook.Events[0].Name)
			assert.Equal(t, "integration-channel", webhook.Events[0].Channel)
			assert.Equal(t, `{"test": "integration"}`, webhook.Events[0].Data)

			// Respond with success
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Integration test successful"))
		}))
		defer server.Close()

		// Create webhook for integration test
		webhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "integration_test",
					Channel: "integration-channel",
					Data:    `{"test": "integration"}`,
				},
			},
		}

		httpWebhook := &HttpWebhook{}

		err := httpWebhook.Send(webhook, server.URL, "integration-test-key", "integration-test-secret")

		assert.NoError(t, err)
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		httpWebhook := &HttpWebhook{}

		// Send multiple concurrent requests
		const numRequests = 10
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(index int) {
				webhook := pusherClient.Webhook{
					TimeMs: int(time.Now().UnixMilli()),
					Events: []pusherClient.WebhookEvent{
						{
							Name:    "concurrent_test",
							Channel: fmt.Sprintf("channel-%d", index),
							Data:    fmt.Sprintf(`{"index": %d}`, index),
						},
					},
				}

				err := httpWebhook.Send(webhook, server.URL, "test-key", "test-secret")
				errors <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
	})
}
