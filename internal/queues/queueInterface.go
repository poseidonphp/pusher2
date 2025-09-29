package queues

import (
	"context"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/webhooks"

	"github.com/pusher/pusher-http-go/v5"
)

type QueueInterface interface {
	addToQueue(data *webhooks.QueuedJobData)
	monitorQueue(ctx context.Context)
	Init() error
	Shutdown(ctx context.Context) // Gracefully shut down the queue and any background goroutines
	Send(app *apps.App, webhook *pusher.WebhookEvent)
	// SendChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64)
	SendClientEvent(app *apps.App, channel constants.ChannelName, event string, data string, socketID constants.SocketID, userID constants.UserID)
	SendMemberAdded(app *apps.App, channel constants.ChannelName, userID constants.UserID)
	SendMemberRemoved(app *apps.App, channel constants.ChannelName, userID constants.UserID)
	SendChannelVacated(app *apps.App, channel constants.ChannelName)
	SendChannelOccupied(app *apps.App, channel constants.ChannelName)
	SendCacheMissed(app *apps.App, channel constants.ChannelName)
}
