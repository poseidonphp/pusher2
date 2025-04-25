package metrics

import (
	"pusher/internal/constants"
	"pusher/internal/payloads"
)

type PrometheusMetrics struct{}

func (p PrometheusMetrics) MarkNewConnection(appId constants.AppID) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkDisconnection(appId constants.AppID) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkApiMessage(appId constants.AppID, incomingMessage *payloads.PusherApiMessage, sentMessage any) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkWsMessageSent(appId constants.AppID, sentMessage any) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkWsMessageReceived(appId constants.AppID, message any) {
	// TODO implement me
}

func (p PrometheusMetrics) TrackHorizontalAdapterResolveTime(appId constants.AppID, time int64) {
	// TODO implement me
}

func (p PrometheusMetrics) TrackHorizontalAdapterResolvedPromises(appId constants.AppID, resolved bool) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkHorizontalAdapterRequestSent(appId constants.AppID) {
	// TODO implement me
}

func (p PrometheusMetrics) MarkHorizontalAdapterRequestReceived(appId constants.AppID) {
	// TODO implement me
}

func (p PrometheusMetrics) GetMetricsAsPlainText() string {
	// TODO implement me
	return ""
}

func (p PrometheusMetrics) GetMetricsAsJson() []byte {
	// TODO implement me
	return nil
}

func (p PrometheusMetrics) Clear() {
	// TODO implement me
}
