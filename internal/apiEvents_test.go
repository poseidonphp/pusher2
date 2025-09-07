package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/metrics"
	"pusher/internal/payloads"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockAdapterForApiEvents is a mock adapter specifically for testing apiEvents.go
type MockAdapterForApiEvents struct {
	*MockAdapter
	sendCalls []SendCall
}

type SendCall struct {
	AppID    constants.AppID
	Channel  constants.ChannelName
	Data     []byte
	SocketID constants.SocketID
}

func NewMockAdapterForApiEvents() *MockAdapterForApiEvents {
	return &MockAdapterForApiEvents{
		MockAdapter: &MockAdapter{
			namespaces: make(map[constants.AppID]*Namespace),
		},
		sendCalls: make([]SendCall, 0),
	}
}

func (m *MockAdapterForApiEvents) Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error {
	var socketID constants.SocketID
	if len(exceptingIds) > 0 {
		socketID = exceptingIds[0]
	}
	m.sendCalls = append(m.sendCalls, SendCall{
		AppID:    appID,
		Channel:  channel,
		Data:     data,
		SocketID: socketID,
	})
	return nil
}

func (m *MockAdapterForApiEvents) GetSendCalls() []SendCall {
	return m.sendCalls
}

func (m *MockAdapterForApiEvents) ClearSendCalls() {
	m.sendCalls = make([]SendCall, 0)
}

// MockCacheManagerForApiEvents is a mock cache manager for testing
type MockCacheManagerForApiEvents struct {
	setExCalls []SetExCall
}

type SetExCall struct {
	Key   string
	Value string
	TTL   time.Duration
}

// We create a mock cache manager so we can track calls to SetEx
func NewMockCacheManagerForApiEvents() *MockCacheManagerForApiEvents {
	return &MockCacheManagerForApiEvents{
		setExCalls: make([]SetExCall, 0),
	}
}

func (m *MockCacheManagerForApiEvents) SetEx(key string, value string, ttl time.Duration) {
	m.setExCalls = append(m.setExCalls, SetExCall{
		Key:   key,
		Value: value,
		TTL:   ttl,
	})
}

func (m *MockCacheManagerForApiEvents) Get(key string) (string, bool) {
	return "", false
}

func (m *MockCacheManagerForApiEvents) Del(key string) error {
	return nil
}

func (m *MockCacheManagerForApiEvents) Delete(key string) {
}

func (m *MockCacheManagerForApiEvents) Init(_ context.Context) error {
	return nil
}

func (m *MockCacheManagerForApiEvents) Set(key string, value string) {
}

func (m *MockCacheManagerForApiEvents) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	return callback()
}

func (m *MockCacheManagerForApiEvents) Has(key string) bool {
	return false
}

func (m *MockCacheManagerForApiEvents) Update(key string, value string) {
}

func (m *MockCacheManagerForApiEvents) GetSetExCalls() []SetExCall {
	return m.setExCalls
}

func (m *MockCacheManagerForApiEvents) ClearSetExCalls() {
	m.setExCalls = make([]SetExCall, 0)
}

// Helper function to create a test app for API events
func createTestAppForApiEvents() *apps.App {
	app := &apps.App{
		ID:                     "test-app",
		Key:                    "test-key",
		Secret:                 "test-secret",
		MaxChannelNameLength:   200,
		MaxEventChannelsAtOnce: 10,
		MaxEventNameLength:     100,
		MaxEventPayloadInKb:    10,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test server for API events
func createTestServerForApiEvents() (*Server, *MockAdapterForApiEvents, *MockCacheManagerForApiEvents) {
	adapter := NewMockAdapterForApiEvents()
	cacheManager := NewMockCacheManagerForApiEvents()
	metricsManager := &metrics.NoOpMetrics{}

	server := &Server{
		Adapter:        adapter,
		CacheManager:   cacheManager,
		MetricsManager: metricsManager,
	}

	return server, adapter, cacheManager
}

func TestCheckMessageToBroadcast(t *testing.T) {
	app := createTestAppForApiEvents()

	t.Run("ValidSingleChannel", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		apiMsg := &payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    "test-data",
			Channel: "public-channel",
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.NoError(t, err)
		assert.Len(t, apiMsg.Channels, 1)
		assert.Equal(t, "public-channel", string(apiMsg.Channels[0]))
	})

	t.Run("ValidMultipleChannels", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: []constants.ChannelName{"public-channel-1", "public-channel-2"},
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.NoError(t, err)
		assert.Len(t, apiMsg.Channels, 2)
	})

	t.Run("InvalidChannel", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		apiMsg := &payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    "test-data",
			Channel: "invalid*channel", // Invalid channel name
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Channel")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("TooManyChannels", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		channels := make([]constants.ChannelName, 11) // More than MaxEventChannelsAtOnce (10)
		for i := 0; i < 11; i++ {
			channels[i] = constants.ChannelName(fmt.Sprintf("channel-%d", i))
		}
		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: channels,
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too many channels")
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("EventNameTooLong", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		longEventName := string(make([]byte, 101)) // Longer than MaxEventNameLength (100)
		apiMsg := &payloads.PusherApiMessage{
			Name:    longEventName,
			Data:    "test-data",
			Channel: "public-channel",
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event name too long")
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("EventDataTooLarge", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		largeData := string(make([]byte, 11*1024)) // Larger than MaxEventPayloadInKb*1024 (10KB)
		apiMsg := &payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    largeData,
			Channel: "public-channel",
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event data too large")
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("EmptyChannelAndChannels", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channel:  "",                        // Empty channel
			Channels: []constants.ChannelName{}, // Empty channels
		}

		err := checkMessageToBroadcast(c, app, apiMsg)

		// Should not error, but channels should remain empty
		assert.NoError(t, err)
		assert.Len(t, apiMsg.Channels, 0)
	})
}

func TestBroadcastMessage(t *testing.T) {
	server, adapter, cacheManager := createTestServerForApiEvents()
	app := createTestAppForApiEvents()

	t.Run("SingleChannel", func(t *testing.T) {
		adapter.ClearSendCalls()
		cacheManager.ClearSetExCalls()

		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: []constants.ChannelName{"public-channel"},
			SocketID: "socket-123",
		}

		broadcastMessage(apiMsg, app, server)

		// Check that Send was called
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)
		assert.Equal(t, app.ID, sendCalls[0].AppID)
		assert.Equal(t, "public-channel", string(sendCalls[0].Channel))
		assert.Equal(t, "socket-123", string(sendCalls[0].SocketID))
		assert.Contains(t, string(sendCalls[0].Data), "test-event")
		assert.Contains(t, string(sendCalls[0].Data), "test-data")

		// Check that cache was not called (not a cache channel)
		setExCalls := cacheManager.GetSetExCalls()
		assert.Len(t, setExCalls, 0)
	})

	t.Run("MultipleChannels", func(t *testing.T) {
		adapter.ClearSendCalls()
		cacheManager.ClearSetExCalls()

		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: []constants.ChannelName{"public-channel-1", "public-channel-2"},
		}

		broadcastMessage(apiMsg, app, server)

		// Check that Send was called for each channel
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 2)
		assert.Equal(t, "public-channel-1", string(sendCalls[0].Channel))
		assert.Equal(t, "public-channel-2", string(sendCalls[1].Channel))
	})

	t.Run("CacheChannel", func(t *testing.T) {
		adapter.ClearSendCalls()
		cacheManager.ClearSetExCalls()

		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: []constants.ChannelName{"cache-public-channel"},
		}

		broadcastMessage(apiMsg, app, server)

		// Check that Send was called
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)

		// Check that cache was called
		setExCalls := cacheManager.GetSetExCalls()
		assert.Len(t, setExCalls, 1)
		assert.Contains(t, setExCalls[0].Key, "test-app")
		assert.Contains(t, setExCalls[0].Key, "cache-public-channel")
		assert.Equal(t, 30*time.Minute, setExCalls[0].TTL)
	})

	t.Run("PresenceCacheChannel", func(t *testing.T) {
		adapter.ClearSendCalls()
		cacheManager.ClearSetExCalls()

		apiMsg := &payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: []constants.ChannelName{"presence-cache-channel"},
		}

		broadcastMessage(apiMsg, app, server)

		// Check that Send was called
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)

		// Check that cache was called
		setExCalls := cacheManager.GetSetExCalls()
		assert.Len(t, setExCalls, 1)
	})
}

func TestEventTrigger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SuccessfulEvent", func(t *testing.T) {
		server, adapter, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		// Create a test context with app in context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create request body
		requestBody := payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    "test-data",
			Channel: "public-channel",
		}
		jsonBody, _ := json.Marshal(requestBody)

		// Create request
		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		// Call the handler
		EventTrigger(c, server)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Empty(t, response) // Should be empty JSON object

		// Check that Send was called
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)
		assert.Equal(t, app.ID, sendCalls[0].AppID)
		assert.Equal(t, "public-channel", string(sendCalls[0].Channel))
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("POST", "/events", nil)
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("POST", "/events", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "invalid character")
	})

	t.Run("ValidationError", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create request with invalid channel
		requestBody := payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    "test-data",
			Channel: "invalid*channel",
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "invalid channel")
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create request with missing required fields
		requestBody := map[string]interface{}{
			"name": "test-event",
			// Missing data and channel
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "required")
	})
}

func TestBatchEventTrigger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SuccessfulBatch", func(t *testing.T) {
		server, adapter, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create batch request
		requestBody := BatchEventForm{
			Batch: []*payloads.PusherApiMessage{
				{
					Name:    "event-1",
					Data:    "data-1",
					Channel: "channel-1",
				},
				{
					Name:    "event-2",
					Data:    "data-2",
					Channel: "channel-2",
				},
			},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/batch_events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		BatchEventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Empty(t, response)

		// Check that Send was called for each event
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 2)
		assert.Equal(t, "channel-1", string(sendCalls[0].Channel))
		assert.Equal(t, "channel-2", string(sendCalls[1].Channel))
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("POST", "/batch_events", nil)
		c.Request = req

		BatchEventTrigger(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("POST", "/batch_events", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		BatchEventTrigger(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "invalid character")
	})

	t.Run("EmptyBatch", func(t *testing.T) {
		server, adapter, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		requestBody := BatchEventForm{
			Batch: []*payloads.PusherApiMessage{},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/batch_events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		BatchEventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that no Send calls were made
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 0)
	})

	t.Run("BatchWithValidationError", func(t *testing.T) {
		server, adapter, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create batch with one valid and one invalid event
		requestBody := BatchEventForm{
			Batch: []*payloads.PusherApiMessage{
				{
					Name:    "valid-event",
					Data:    "valid-data",
					Channel: "valid-channel",
				},
				{
					Name:    "invalid-event",
					Data:    "invalid-data",
					Channel: "invalid*channel", // Invalid channel
				},
			},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/batch_events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		BatchEventTrigger(c, server)

		// Should return 400 due to validation error
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "invalid channel")

		// Check that only the valid event was processed
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)
		assert.Equal(t, "valid-channel", string(sendCalls[0].Channel))
	})
}

func TestApiEvents_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ChannelNameAtLimit", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create channel name at the limit
		longChannelName := ""
		for i := 0; i < app.MaxChannelNameLength; i++ {
			longChannelName += "a"
		}
		requestBody := payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    "test-data",
			Channel: constants.ChannelName(longChannelName),
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("EventNameAtLimit", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create event name at the limit
		longEventName := string(make([]byte, app.MaxEventNameLength))
		requestBody := payloads.PusherApiMessage{
			Name:    longEventName,
			Data:    "test-data",
			Channel: "public-channel",
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("EventDataAtLimit", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create data at the limit
		largeData := string(make([]byte, app.MaxEventPayloadInKb*1024))
		requestBody := payloads.PusherApiMessage{
			Name:    "test-event",
			Data:    largeData,
			Channel: "public-channel",
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ChannelsAtLimit", func(t *testing.T) {
		server, _, _ := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create channels at the limit
		channels := make([]constants.ChannelName, app.MaxEventChannelsAtOnce)
		for i := 0; i < app.MaxEventChannelsAtOnce; i++ {
			channels[i] = constants.ChannelName(fmt.Sprintf("channel-%d", i))
		}
		requestBody := payloads.PusherApiMessage{
			Name:     "test-event",
			Data:     "test-data",
			Channels: channels,
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestApiEvents_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("FullWorkflowWithCache", func(t *testing.T) {
		server, adapter, cacheManager := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Test with cache channel
		requestBody := payloads.PusherApiMessage{
			Name:     "cache-event",
			Data:     "cache-data",
			Channel:  "cache-public-channel",
			SocketID: "socket-123",
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		EventTrigger(c, server)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// Check that Send was called
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 1)
		assert.Equal(t, "socket-123", string(sendCalls[0].SocketID))

		// Check that cache was called
		setExCalls := cacheManager.GetSetExCalls()
		assert.Len(t, setExCalls, 1)
		assert.Equal(t, 30*time.Minute, setExCalls[0].TTL)
	})

	t.Run("BatchWithMixedChannelTypes", func(t *testing.T) {
		server, adapter, cacheManager := createTestServerForApiEvents()
		app := createTestAppForApiEvents()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		// Create batch with different channel types
		requestBody := BatchEventForm{
			Batch: []*payloads.PusherApiMessage{
				{
					Name:    "public-event",
					Data:    "public-data",
					Channel: "public-channel",
				},
				{
					Name:    "cache-event",
					Data:    "cache-data",
					Channel: "cache-public-channel",
				},
				{
					Name:    "presence-cache-event",
					Data:    "presence-cache-data",
					Channel: "presence-cache-channel",
				},
			},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/batch_events", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		BatchEventTrigger(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check that Send was called for all events
		sendCalls := adapter.GetSendCalls()
		assert.Len(t, sendCalls, 3)

		// Check that cache was called for cache channels
		setExCalls := cacheManager.GetSetExCalls()
		assert.Len(t, setExCalls, 2) // cache-public-channel and presence-cache-channel
	})
}
