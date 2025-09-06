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
	SetChannelsTotal(count float64)
	SetPresenceChannels(count float64)
	SetPrivateChannels(count float64)
	SetPublicChannels(count float64)
	MarkError(errorType string, appId constants.AppID)
	GetMetricsAsPlainText() string
	GetMetricsAsJson() []byte // this should return an instance of prometheus metrics (.Collector?)
	Clear()
}

type NoOpMetrics struct {
}

func (n NoOpMetrics) MarkNewConnection(_ constants.AppID) {
	return
}

func (n NoOpMetrics) MarkDisconnection(_ constants.AppID) {
	return
}

func (n NoOpMetrics) MarkApiMessage(_ constants.AppID, _ *payloads.PusherApiMessage, _ any) {
	return
}

func (n NoOpMetrics) MarkWsMessageSent(_ constants.AppID, _ any) {
	return
}

func (n NoOpMetrics) MarkWsMessageReceived(_ constants.AppID, _ any) {
	return
}

func (n NoOpMetrics) TrackHorizontalAdapterResolveTime(_ constants.AppID, _ int64) {
	return
}

func (n NoOpMetrics) TrackHorizontalAdapterResolvedPromises(_ constants.AppID, _ bool) {
	return
}

func (n NoOpMetrics) MarkHorizontalAdapterRequestSent(_ constants.AppID) {
	return
}

func (n NoOpMetrics) MarkHorizontalAdapterRequestReceived(_ constants.AppID) {
	return
}

func (n NoOpMetrics) SetChannelsTotal(_ float64) {
	return
}

func (n NoOpMetrics) SetPresenceChannels(_ float64) {
	return
}

func (n NoOpMetrics) SetPrivateChannels(_ float64) {
	return
}

func (n NoOpMetrics) SetPublicChannels(_ float64) {
	return
}

func (n NoOpMetrics) MarkError(_ string, _ constants.AppID) {
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
