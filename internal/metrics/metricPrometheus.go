package metrics

import (
	"bytes"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	"pusher/internal/constants"
	"pusher/internal/payloads"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/expfmt"
)

// PrometheusMetrics implements the MetricsInterface using Prometheus metrics
type PrometheusMetrics struct {
	// Configuration
	namespace     string
	subsystem     string
	labels        map[string]string
	customMetrics map[string]prometheus.Collector
	customMutex   sync.RWMutex

	// Connection metrics
	connectionsTotal    prometheus.Counter
	disconnectionsTotal prometheus.Counter
	activeConnections   prometheus.Gauge

	// API message metrics
	apiMessagesTotal  prometheus.Counter
	apiMessagesFailed prometheus.Counter
	apiMessageLatency prometheus.Histogram

	// WebSocket message metrics
	wsMessagesSentTotal     prometheus.Counter
	wsMessagesReceivedTotal prometheus.Counter
	wsMessageLatency        prometheus.Histogram

	// Horizontal adapter metrics
	horizontalAdapterRequestsTotal    prometheus.Counter
	horizontalAdapterRequestsReceived prometheus.Counter
	horizontalAdapterResolveTime      prometheus.Histogram
	horizontalAdapterResolvedPromises prometheus.Counter
	horizontalAdapterFailedPromises   prometheus.Counter

	// Channel metrics
	channelsTotal    prometheus.Gauge
	presenceChannels prometheus.Gauge
	privateChannels  prometheus.Gauge
	publicChannels   prometheus.Gauge

	// Event metrics
	eventsTotal     prometheus.Counter
	eventsByType    *prometheus.CounterVec
	eventsByChannel *prometheus.CounterVec

	// Error metrics
	errorsTotal  prometheus.Counter
	errorsByType *prometheus.CounterVec

	// Performance metrics
	responseTime prometheus.Histogram
	requestSize  prometheus.Histogram
	responseSize prometheus.Histogram

	// Memory and resource metrics
	memoryUsage     prometheus.Gauge
	cpuUsage        prometheus.Gauge
	goroutinesTotal prometheus.Gauge
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance
func NewPrometheusMetrics(namespace, subsystem string) *PrometheusMetrics {
	pm := &PrometheusMetrics{
		namespace:     namespace,
		subsystem:     subsystem,
		labels:        make(map[string]string),
		customMetrics: make(map[string]prometheus.Collector),
	}

	// Initialize all metrics
	pm.initializeMetrics()

	return pm
}

// NewPrometheusMetricsWithLabels creates a new PrometheusMetrics instance with custom labels
func NewPrometheusMetricsWithLabels(namespace, subsystem string, labels map[string]string) *PrometheusMetrics {
	pm := NewPrometheusMetrics(namespace, subsystem)
	pm.labels = labels
	return pm
}

func (pm *PrometheusMetrics) initializeMetrics() {
	// Connection metrics
	pm.connectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "connections_total",
		Help:        "Total number of connections established",
		ConstLabels: pm.labels,
	})

	pm.disconnectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "disconnections_total",
		Help:        "Total number of disconnections",
		ConstLabels: pm.labels,
	})

	pm.activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "active_connections",
		Help:        "Current number of active connections",
		ConstLabels: pm.labels,
	})

	// API message metrics
	pm.apiMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "api_messages_total",
		Help:        "Total number of API messages processed",
		ConstLabels: pm.labels,
	})

	pm.apiMessagesFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "api_messages_failed_total",
		Help:        "Total number of failed API messages",
		ConstLabels: pm.labels,
	})

	pm.apiMessageLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "api_message_duration_seconds",
		Help:        "API message processing duration in seconds",
		Buckets:     prometheus.DefBuckets,
		ConstLabels: pm.labels,
	})

	// WebSocket message metrics
	pm.wsMessagesSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "ws_messages_sent_total",
		Help:        "Total number of WebSocket messages sent",
		ConstLabels: pm.labels,
	})

	pm.wsMessagesReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "ws_messages_received_total",
		Help:        "Total number of WebSocket messages received",
		ConstLabels: pm.labels,
	})

	pm.wsMessageLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "ws_message_duration_seconds",
		Help:        "WebSocket message processing duration in seconds",
		Buckets:     prometheus.DefBuckets,
		ConstLabels: pm.labels,
	})

	// Horizontal adapter metrics
	pm.horizontalAdapterRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "horizontal_adapter_requests_total",
		Help:        "Total number of horizontal adapter requests sent",
		ConstLabels: pm.labels,
	})

	pm.horizontalAdapterRequestsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "horizontal_adapter_requests_received_total",
		Help:        "Total number of horizontal adapter requests received",
		ConstLabels: pm.labels,
	})

	pm.horizontalAdapterResolveTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "horizontal_adapter_resolve_duration_seconds",
		Help:        "Horizontal adapter resolve time in seconds",
		Buckets:     prometheus.DefBuckets,
		ConstLabels: pm.labels,
	})

	pm.horizontalAdapterResolvedPromises = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "horizontal_adapter_resolved_promises_total",
		Help:        "Total number of resolved horizontal adapter promises",
		ConstLabels: pm.labels,
	})

	pm.horizontalAdapterFailedPromises = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "horizontal_adapter_failed_promises_total",
		Help:        "Total number of failed horizontal adapter promises",
		ConstLabels: pm.labels,
	})

	// Channel metrics
	pm.channelsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "channels_total",
		Help:        "Total number of channels",
		ConstLabels: pm.labels,
	})

	pm.presenceChannels = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "presence_channels_total",
		Help:        "Total number of presence channels",
		ConstLabels: pm.labels,
	})

	pm.privateChannels = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "private_channels_total",
		Help:        "Total number of private channels",
		ConstLabels: pm.labels,
	})

	pm.publicChannels = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "public_channels_total",
		Help:        "Total number of public channels",
		ConstLabels: pm.labels,
	})

	// Event metrics
	pm.eventsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "events_total",
		Help:        "Total number of events processed",
		ConstLabels: pm.labels,
	})

	pm.eventsByType = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "events_by_type_total",
		Help:        "Total number of events by type",
		ConstLabels: pm.labels,
	}, []string{"event_type", "app_id"})

	pm.eventsByChannel = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "events_by_channel_total",
		Help:        "Total number of events by channel",
		ConstLabels: pm.labels,
	}, []string{"channel", "app_id"})

	// Error metrics
	pm.errorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "errors_total",
		Help:        "Total number of errors",
		ConstLabels: pm.labels,
	})

	pm.errorsByType = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "errors_by_type_total",
		Help:        "Total number of errors by type",
		ConstLabels: pm.labels,
	}, []string{"error_type", "app_id"})

	// Performance metrics
	pm.responseTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "response_duration_seconds",
		Help:        "Response time in seconds",
		Buckets:     prometheus.DefBuckets,
		ConstLabels: pm.labels,
	})

	pm.requestSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "request_size_bytes",
		Help:        "Request size in bytes",
		Buckets:     prometheus.ExponentialBuckets(100, 10, 8),
		ConstLabels: pm.labels,
	})

	pm.responseSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "response_size_bytes",
		Help:        "Response size in bytes",
		Buckets:     prometheus.ExponentialBuckets(100, 10, 8),
		ConstLabels: pm.labels,
	})

	// Memory and resource metrics
	pm.memoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "memory_usage_bytes",
		Help:        "Current memory usage in bytes",
		ConstLabels: pm.labels,
	})

	pm.cpuUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "cpu_usage_percent",
		Help:        "Current CPU usage percentage",
		ConstLabels: pm.labels,
	})

	pm.goroutinesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   pm.namespace,
		Subsystem:   pm.subsystem,
		Name:        "goroutines_total",
		Help:        "Current number of goroutines",
		ConstLabels: pm.labels,
	})
}

func (pm *PrometheusMetrics) MarkNewConnection(appId constants.AppID) {
	pm.connectionsTotal.Inc()
	pm.activeConnections.Inc()
}

func (pm *PrometheusMetrics) MarkDisconnection(appId constants.AppID) {
	pm.disconnectionsTotal.Inc()
	pm.activeConnections.Dec()
}

func (pm *PrometheusMetrics) MarkApiMessage(appId constants.AppID, incomingMessage *payloads.PusherApiMessage, sentMessage any) {
	pm.apiMessagesTotal.Inc()

	// Record event by type
	if incomingMessage != nil {
		pm.eventsByType.WithLabelValues(incomingMessage.Name, string(appId)).Inc()
		pm.eventsByChannel.WithLabelValues(incomingMessage.Channel, string(appId)).Inc()
	}

	pm.eventsTotal.Inc()
}

func (pm *PrometheusMetrics) MarkWsMessageSent(appId constants.AppID, sentMessage any) {
	pm.wsMessagesSentTotal.Inc()
}

func (pm *PrometheusMetrics) MarkWsMessageReceived(appId constants.AppID, message any) {
	pm.wsMessagesReceivedTotal.Inc()
}

func (pm *PrometheusMetrics) TrackHorizontalAdapterResolveTime(appId constants.AppID, time int64) {
	pm.horizontalAdapterResolveTime.Observe(float64(time) / 1000000000.0) // Convert nanoseconds to seconds
}

func (pm *PrometheusMetrics) TrackHorizontalAdapterResolvedPromises(appId constants.AppID, resolved bool) {
	if resolved {
		pm.horizontalAdapterResolvedPromises.Inc()
	} else {
		pm.horizontalAdapterFailedPromises.Inc()
	}
}

func (pm *PrometheusMetrics) MarkHorizontalAdapterRequestSent(appId constants.AppID) {
	pm.horizontalAdapterRequestsTotal.Inc()
}

func (pm *PrometheusMetrics) MarkHorizontalAdapterRequestReceived(appId constants.AppID) {
	pm.horizontalAdapterRequestsReceived.Inc()
}

func (pm *PrometheusMetrics) GetMetricsAsPlainText() string {
	// Gather all metrics from the default registry
	gatherer := prometheus.DefaultGatherer

	// Collect all metrics
	metricFamilies, err := gatherer.Gather()
	if err != nil {
		return "# Error gathering metrics: " + err.Error()
	}

	// Create a buffer to write the metrics
	var buf bytes.Buffer

	// Write metrics in Prometheus exposition format
	for _, mf := range metricFamilies {
		_, _ = expfmt.MetricFamilyToText(&buf, mf)
	}

	return buf.String()
}

func (pm *PrometheusMetrics) GetMetricsAsJson() []byte {
	// Collect all metrics and return as JSON
	metrics := make(map[string]interface{})

	// This is a simplified version - in practice, you'd collect from the registry
	metrics["timestamp"] = time.Now().Unix()
	metrics["namespace"] = pm.namespace
	metrics["subsystem"] = pm.subsystem

	jsonData, _ := json.Marshal(metrics)
	return jsonData
}

func (pm *PrometheusMetrics) Clear() {
	// Note: Prometheus metrics are typically not cleared in production
	// This is mainly for testing purposes
	pm.activeConnections.Set(0)
	pm.channelsTotal.Set(0)
	pm.presenceChannels.Set(0)
	pm.privateChannels.Set(0)
	pm.publicChannels.Set(0)
}

// Additional helper methods for more granular control

// SetActiveConnections sets the current number of active connections
func (pm *PrometheusMetrics) SetActiveConnections(count float64) {
	pm.activeConnections.Set(count)
}

// SetChannelsTotal sets the total number of channels
func (pm *PrometheusMetrics) SetChannelsTotal(count float64) {
	pm.channelsTotal.Set(count)
}

// SetPresenceChannels sets the number of presence channels
func (pm *PrometheusMetrics) SetPresenceChannels(count float64) {
	pm.presenceChannels.Set(count)
}

// SetPrivateChannels sets the number of private channels
func (pm *PrometheusMetrics) SetPrivateChannels(count float64) {
	pm.privateChannels.Set(count)
}

// SetPublicChannels sets the number of public channels
func (pm *PrometheusMetrics) SetPublicChannels(count float64) {
	pm.publicChannels.Set(count)
}

// MarkError increments error counters
func (pm *PrometheusMetrics) MarkError(errorType string, appId constants.AppID) {
	pm.errorsTotal.Inc()
	pm.errorsByType.WithLabelValues(errorType, string(appId)).Inc()
}

// ObserveResponseTime records response time
func (pm *PrometheusMetrics) ObserveResponseTime(duration time.Duration) {
	pm.responseTime.Observe(duration.Seconds())
}

// ObserveRequestSize records request size
func (pm *PrometheusMetrics) ObserveRequestSize(size int) {
	pm.requestSize.Observe(float64(size))
}

// ObserveResponseSize records response size
func (pm *PrometheusMetrics) ObserveResponseSize(size int) {
	pm.responseSize.Observe(float64(size))
}

// UpdateResourceMetrics updates memory, CPU, and goroutine metrics
func (pm *PrometheusMetrics) UpdateResourceMetrics() {
	// This would typically be called periodically to update resource metrics
	// Implementation would depend on your specific needs
	pm.goroutinesTotal.Set(float64(runtime.NumGoroutine()))
}

// AddCustomMetric adds a custom metric
func (pm *PrometheusMetrics) AddCustomMetric(name string, collector prometheus.Collector) {
	pm.customMutex.Lock()
	defer pm.customMutex.Unlock()
	pm.customMetrics[name] = collector
}

// GetCustomMetric retrieves a custom metric
func (pm *PrometheusMetrics) GetCustomMetric(name string) (prometheus.Collector, bool) {
	pm.customMutex.RLock()
	defer pm.customMutex.RUnlock()
	collector, exists := pm.customMetrics[name]
	return collector, exists
}

// GetNamespace returns the metrics namespace
func (pm *PrometheusMetrics) GetNamespace() string {
	return pm.namespace
}

// GetSubsystem returns the metrics subsystem
func (pm *PrometheusMetrics) GetSubsystem() string {
	return pm.subsystem
}

// GetLabels returns the metrics labels
func (pm *PrometheusMetrics) GetLabels() map[string]string {
	return pm.labels
}
