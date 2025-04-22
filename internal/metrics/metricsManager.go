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
