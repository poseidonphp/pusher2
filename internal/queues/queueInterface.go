package queues

import (
	"context"

	"github.com/pusher/pusher-http-go/v5"
	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/webhooks"
)

type QueueInterface interface {
	addToQueue(data *webhooks.QueuedJobData)
	monitorQueue(ctx context.Context)
	Init() error
	Send(app *apps.App, webhook *pusher.WebhookEvent)
	// SendChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64)
	SendClientEvent(app *apps.App, channel constants.ChannelName, event string, data string, socketID constants.SocketID, userID constants.UserID)
	SendMemberAdded(app *apps.App, channel constants.ChannelName, userID constants.UserID)
	SendMemberRemoved(app *apps.App, channel constants.ChannelName, userID constants.UserID)
	SendChannelVacated(app *apps.App, channel constants.ChannelName)
	SendChannelOccupied(app *apps.App, channel constants.ChannelName)
	SendCacheMissed(app *apps.App, channel constants.ChannelName)
}
