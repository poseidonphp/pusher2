package metrics

import (
	"testing"
	"time"

	"pusher/internal/constants"
	"pusher/internal/payloads"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// createTestPrometheusMetrics creates a PrometheusMetrics instance for testing
// without using promauto to avoid global registry conflicts
func createTestPrometheusMetrics() *PrometheusMetrics {
	pm := &PrometheusMetrics{
		labels:        make(map[string]string),
		customMetrics: make(map[string]prometheus.Collector),
	}

	// Initialize metrics manually without promauto
	pm.connectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "connections_total",
		Help: "Total number of connections established",
	})

	pm.disconnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "disconnections_total",
		Help: "Total number of disconnections",
	})

	pm.activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "active_connections",
		Help: "Current number of active connections",
	})

	pm.apiMessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_messages_total",
		Help: "Total number of API messages processed",
	})

	pm.wsMessagesSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ws_messages_sent_total",
		Help: "Total number of WebSocket messages sent",
	})

	pm.wsMessagesReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ws_messages_received_total",
		Help: "Total number of WebSocket messages received",
	})

	pm.horizontalAdapterRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "horizontal_adapter_requests_total",
		Help: "Total number of horizontal adapter requests sent",
	})

	pm.horizontalAdapterRequestsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "horizontal_adapter_requests_received_total",
		Help: "Total number of horizontal adapter requests received",
	})

	pm.horizontalAdapterResolveTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "horizontal_adapter_resolve_duration_seconds",
		Help:    "Horizontal adapter resolve time in seconds",
		Buckets: prometheus.DefBuckets,
	})

	pm.horizontalAdapterResolvedPromises = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "horizontal_adapter_resolved_promises_total",
		Help: "Total number of resolved horizontal adapter promises",
	})

	pm.horizontalAdapterFailedPromises = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "horizontal_adapter_failed_promises_total",
		Help: "Total number of failed horizontal adapter promises",
	})

	pm.channelsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "channels_total",
		Help: "Total number of channels",
	})

	pm.presenceChannels = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "presence_channels_total",
		Help: "Total number of presence channels",
	})

	pm.privateChannels = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "private_channels_total",
		Help: "Total number of private channels",
	})

	pm.publicChannels = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "public_channels_total",
		Help: "Total number of public channels",
	})

	pm.eventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "events_total",
		Help: "Total number of events processed",
	})

	pm.eventsByType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_by_type_total",
		Help: "Total number of events by type",
	}, []string{"event_type", "app_id"})

	pm.eventsByChannel = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_by_channel_total",
		Help: "Total number of events by channel",
	}, []string{"channel", "app_id"})

	pm.errorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "errors_total",
		Help: "Total number of errors",
	})

	pm.errorsByType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "errors_by_type_total",
		Help: "Total number of errors by type",
	}, []string{"error_type", "app_id"})

	pm.responseTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "response_duration_seconds",
		Help:    "Response time in seconds",
		Buckets: prometheus.DefBuckets,
	})

	pm.requestSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "request_size_bytes",
		Help:    "Request size in bytes",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	})

	pm.responseSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "response_size_bytes",
		Help:    "Response size in bytes",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	})

	pm.memoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})

	pm.cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})

	pm.goroutinesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goroutines_total",
		Help: "Current number of goroutines",
	})

	return pm
}

func TestNewPrometheusMetrics(t *testing.T) {
	t.Run("BasicCreation", func(t *testing.T) {
		pm := createTestPrometheusMetrics()

		assert.NotNil(t, pm)
		assert.NotNil(t, pm.GetLabels())
	})

	t.Run("WithLabels", func(t *testing.T) {
		labels := map[string]string{
			"environment": "test",
			"version":     "1.0.0",
		}
		pm := createTestPrometheusMetrics()
		pm.labels = labels

		assert.NotNil(t, pm)
		assert.Equal(t, labels, pm.GetLabels())
	})
}

func TestPrometheusMetrics_ConnectionMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("MarkNewConnection", func(t *testing.T) {
		pm.MarkNewConnection(appId)
		// Note: We can't easily test the internal counter values without exposing them
		// In a real implementation, you might want to add getter methods for testing
	})

	t.Run("MarkDisconnection", func(t *testing.T) {
		pm.MarkDisconnection(appId)
	})

	t.Run("SetActiveConnections", func(t *testing.T) {
		pm.SetActiveConnections(42.0)
	})
}

func TestPrometheusMetrics_ApiMessageMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("MarkApiMessage", func(t *testing.T) {
		incomingMessage := &payloads.PusherApiMessage{
			Name:    "test-event",
			Channel: "test-channel",
		}

		pm.MarkApiMessage(appId, incomingMessage, "sent-message")
	})

	t.Run("MarkApiMessageWithNil", func(t *testing.T) {
		pm.MarkApiMessage(appId, nil, "sent-message")
	})
}

func TestPrometheusMetrics_WebSocketMessageMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("MarkWsMessageSent", func(t *testing.T) {
		pm.MarkWsMessageSent(appId, "test-message")
	})

	t.Run("MarkWsMessageReceived", func(t *testing.T) {
		pm.MarkWsMessageReceived(appId, "test-message")
	})
}

func TestPrometheusMetrics_HorizontalAdapterMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("TrackHorizontalAdapterResolveTime", func(t *testing.T) {
		// Test with 1 second in nanoseconds
		pm.TrackHorizontalAdapterResolveTime(appId, 1000000000)
	})

	t.Run("TrackHorizontalAdapterResolvedPromises", func(t *testing.T) {
		pm.TrackHorizontalAdapterResolvedPromises(appId, true)
		pm.TrackHorizontalAdapterResolvedPromises(appId, false)
	})

	t.Run("MarkHorizontalAdapterRequestSent", func(t *testing.T) {
		pm.MarkHorizontalAdapterRequestSent(appId)
	})

	t.Run("MarkHorizontalAdapterRequestReceived", func(t *testing.T) {
		pm.MarkHorizontalAdapterRequestReceived(appId)
	})
}

func TestPrometheusMetrics_ChannelMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("SetChannelCounts", func(t *testing.T) {
		pm.SetChannelsTotal(100.0)
		pm.SetPresenceChannels(10.0)
		pm.SetPrivateChannels(30.0)
		pm.SetPublicChannels(60.0)
	})
}

func TestPrometheusMetrics_ErrorMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("MarkError", func(t *testing.T) {
		pm.MarkError("connection_error", appId)
		pm.MarkError("auth_error", appId)
	})
}

func TestPrometheusMetrics_PerformanceMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("ObserveResponseTime", func(t *testing.T) {
		pm.ObserveResponseTime(100 * time.Millisecond)
		pm.ObserveResponseTime(500 * time.Millisecond)
	})

	t.Run("ObserveRequestSize", func(t *testing.T) {
		pm.ObserveRequestSize(1024)
		pm.ObserveRequestSize(2048)
	})

	t.Run("ObserveResponseSize", func(t *testing.T) {
		pm.ObserveResponseSize(512)
		pm.ObserveResponseSize(1024)
	})
}

func TestPrometheusMetrics_ResourceMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("UpdateResourceMetrics", func(t *testing.T) {
		pm.UpdateResourceMetrics()
	})
}

func TestPrometheusMetrics_CustomMetrics(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("AddCustomMetric", func(t *testing.T) {
		// Create a simple counter for testing
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_custom_counter",
			Help: "A test custom counter",
		})

		pm.AddCustomMetric("test_counter", counter)

		// Verify it was added
		retrieved, exists := pm.GetCustomMetric("test_counter")
		assert.True(t, exists)
		assert.Equal(t, counter, retrieved)
	})

	t.Run("GetNonExistentCustomMetric", func(t *testing.T) {
		_, exists := pm.GetCustomMetric("non_existent")
		assert.False(t, exists)
	})
}

func TestPrometheusMetrics_UtilityMethods(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("GetMetricsAsPlainText", func(t *testing.T) {
		text := pm.GetMetricsAsPlainText()
		assert.Contains(t, text, "go_goroutines") // Example metric from Go runtime
		assert.Greater(t, len(text), 0)
	})

	t.Run("GetMetricsAsJson", func(t *testing.T) {
		jsonData := pm.GetMetricsAsJson()
		assert.NotNil(t, jsonData)
		assert.Greater(t, len(jsonData), 0)
	})

	t.Run("Clear", func(t *testing.T) {
		// Set some values first
		pm.SetActiveConnections(10.0)
		pm.SetChannelsTotal(5.0)

		// Clear them
		pm.Clear()

		// Note: We can't easily verify the internal state without getter methods
		// In a real implementation, you might want to add getter methods for testing
	})
}

func TestPrometheusMetrics_ConcurrentAccess(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("ConcurrentConnections", func(t *testing.T) {
		// Simulate concurrent connections and disconnections
		for i := 0; i < 100; i++ {
			go func() {
				pm.MarkNewConnection(appId)
				pm.MarkDisconnection(appId)
			}()
		}

		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("ConcurrentMessages", func(t *testing.T) {
		// Simulate concurrent message processing
		for i := 0; i < 100; i++ {
			go func() {
				pm.MarkWsMessageSent(appId, "test-message")
				pm.MarkWsMessageReceived(appId, "test-message")
			}()
		}

		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	})
}

func TestPrometheusMetrics_EdgeCases(t *testing.T) {
	pm := createTestPrometheusMetrics()

	t.Run("EmptyAppId", func(t *testing.T) {
		emptyAppId := constants.AppID("")
		pm.MarkNewConnection(emptyAppId)
		pm.MarkDisconnection(emptyAppId)
	})

	t.Run("NegativeValues", func(t *testing.T) {
		// Test that negative values are handled gracefully
		pm.SetActiveConnections(-1.0)
		pm.SetChannelsTotal(-5.0)
	})

	t.Run("VeryLargeValues", func(t *testing.T) {
		// Test with very large values
		pm.SetActiveConnections(1e6)
		pm.SetChannelsTotal(1e6)
	})
}

func TestPrometheusMetrics_LabelValues(t *testing.T) {
	pm := createTestPrometheusMetrics()
	appId := constants.AppID("test-app")

	t.Run("DifferentEventTypes", func(t *testing.T) {
		// Test different event types
		eventTypes := []string{"subscribe", "unsubscribe", "client_event", "pusher:ping", "pusher:pong"}

		for _, eventType := range eventTypes {
			incomingMessage := &payloads.PusherApiMessage{
				Name:    eventType,
				Channel: "test-channel",
			}
			pm.MarkApiMessage(appId, incomingMessage, "sent-message")
		}
	})

	t.Run("DifferentChannels", func(t *testing.T) {
		// Test different channel types
		channels := []string{"public-channel", "private-channel", "presence-channel"}

		for _, channel := range channels {
			incomingMessage := &payloads.PusherApiMessage{
				Name:    "test-event",
				Channel: channel,
			}
			pm.MarkApiMessage(appId, incomingMessage, "sent-message")
		}
	})

	t.Run("DifferentErrorTypes", func(t *testing.T) {
		// Test different error types
		errorTypes := []string{"auth_error", "connection_error", "rate_limit_error", "invalid_channel"}

		for _, errorType := range errorTypes {
			pm.MarkError(errorType, appId)
		}
	})
}
