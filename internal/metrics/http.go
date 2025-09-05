package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler provides HTTP handlers for exposing Prometheus metrics
type MetricsHandler struct {
	metrics *PrometheusMetrics
}

// NewMetricsHandler creates a new MetricsHandler
func NewMetricsHandler(metrics *PrometheusMetrics) *MetricsHandler {
	return &MetricsHandler{
		metrics: metrics,
	}
}

// PrometheusHandler returns an HTTP handler for Prometheus metrics
func (mh *MetricsHandler) PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// JSONHandler returns an HTTP handler for JSON metrics
func (mh *MetricsHandler) JSONHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mh.metrics.GetMetricsAsJson())
	})
}

// TextHandler returns an HTTP handler for plain text metrics
func (mh *MetricsHandler) TextHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mh.metrics.GetMetricsAsPlainText()))
	})
}

// SetupMetricsRoutes sets up common metrics routes on an HTTP mux
func (mh *MetricsHandler) SetupMetricsRoutes(mux *http.ServeMux) {
	mux.Handle("/metrics", mh.PrometheusHandler())
	mux.Handle("/metrics/json", mh.JSONHandler())
	mux.Handle("/metrics/text", mh.TextHandler())
}
