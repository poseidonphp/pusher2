package metrics

import (
	"pusher/internal/constants"
	"pusher/internal/payloads"
)

type MetricsInterface interface {
	MarkNewConnection(appId constants.AppID)
	MarkDisconnection(appId constants.AppID)
	MarkApiMessage(appId constants.AppID, incomingMessage *payloads.PusherApiMessage, sentMessage any)
	MarkWsMessageSent(appId constants.AppID, sentMessage any)
	MarkWsMessageReceived(appId constants.AppID, message any)
	TrackHorizontalAdapterResolveTime(appId constants.AppID, time int64)
	TrackHorizontalAdapterResolvedPromises(appId constants.AppID, resolved bool)
	MarkHorizontalAdapterRequestSent(appId constants.AppID)
	MarkHorizontalAdapterRequestReceived(appId constants.AppID)
	GetMetricsAsPlainText() string
	GetMetricsAsJson() []byte // this should return an instance of prometheus metrics (.Collector?)
	Clear()
}

type NoOpMetrics struct {
}

func (n NoOpMetrics) MarkNewConnection(appId constants.AppID) {
	return
}

func (n NoOpMetrics) MarkDisconnection(appId constants.AppID) {
	return
}

func (n NoOpMetrics) MarkApiMessage(appId constants.AppID, incomingMessage *payloads.PusherApiMessage, sentMessage any) {
	return
}

func (n NoOpMetrics) MarkWsMessageSent(appId constants.AppID, sentMessage any) {
	return
}

func (n NoOpMetrics) MarkWsMessageReceived(appId constants.AppID, message any) {
	return
}

func (n NoOpMetrics) TrackHorizontalAdapterResolveTime(appId constants.AppID, time int64) {
	return
}

func (n NoOpMetrics) TrackHorizontalAdapterResolvedPromises(appId constants.AppID, resolved bool) {
	return
}

func (n NoOpMetrics) MarkHorizontalAdapterRequestSent(appId constants.AppID) {
	return
}

func (n NoOpMetrics) MarkHorizontalAdapterRequestReceived(appId constants.AppID) {
	return
}

func (n NoOpMetrics) GetMetricsAsPlainText() string {
	return ""
}

func (n NoOpMetrics) GetMetricsAsJson() []byte {
	return []byte("{}")
}

func (n NoOpMetrics) Clear() {
	return
}
