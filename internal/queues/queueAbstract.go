package queues

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pusher/pusher-http-go/v5"
	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/webhooks"
)

func NewAbstractQueue(ctx context.Context, queueImpl QueueInterface, webhookSender *webhooks.WebhookSender, enableFlapDetection bool, flapDetectionWindow time.Duration) (*AbstractQueue, error) {
	if queueImpl == nil {
		return nil, errors.New("queue implementation is nil")
	}
	fd := &FlapDetector{
		FlapEnabled:         enableFlapDetection,
		flapWindowInSeconds: flapDetectionWindow,
	}
	fd.Init()

	q := &AbstractQueue{
		flapDetector:  fd,
		webhookSender: webhookSender,
	}
	err := queueImpl.Init()
	if err != nil {
		return nil, err
	}
	q.concreteQueue = queueImpl

	// Start the queue monitoring in a separate goroutine
	go q.concreteQueue.monitorQueue(ctx)

	return q, nil
}

type AbstractQueue struct {
	flapDetector   *FlapDetector
	webhookSender  *webhooks.WebhookSender
	concreteQueue  QueueInterface
	batch          []*pusher.WebhookEvent
	batchHasLeader bool
}

// Send should be used by parts of the app when wanting to send a webhook.
// It will take care of sending it for flap detection if necessary
func (q *AbstractQueue) Send(app *apps.App, event *pusher.WebhookEvent) {
	if q.webhookSender == nil || !app.WebhooksEnabled {
		return
	}

	// Run through flap detection
	// Run through other things like rate limiter?

	// Determine the EventType (connect or disconnect) based on the event name and switch/case choices
	// we will create a key that we can use for flap detection
	var eventType EventType
	var key string
	useFlapDetection := true

	if q.flapDetector.FlapEnabled {
		switch event.Name {
		case string(constants.WebHookChannelOccupied):
			eventType = Connect
			key = fmt.Sprintf("app:%s:channel:%s", app.ID, event.Channel)
		case string(constants.WebHookChannelVacated):
			eventType = Disconnect
			key = fmt.Sprintf("app:%s:channel:%s", app.ID, event.Channel)
		case string(constants.WebHookMemberAdded):
			eventType = Connect
			key = fmt.Sprintf("app:%s:channel:%s:member:%s", app.ID, event.Channel, event.UserID)
		case string(constants.WebHookMemberRemoved):
			eventType = Disconnect
			key = fmt.Sprintf("app:%s:channel:%s:member:%s", app.ID, event.Channel, event.UserID)
		default:
			// things like cache_miss, client_event, subscription_count
			useFlapDetection = false
		}
	} else {
		useFlapDetection = false
	}

	if useFlapDetection {
		// process via flap detection. If the messages should be sent, it will call the addToQueue method
		q.flapDetector.CheckForFlapping(app, key, eventType, event, q.prepareQueuedMessages)
	} else {
		// skip flap detection, send right to the queue
		q.prepareQueuedMessages(app, event)
	}
}

func (q *AbstractQueue) prepareQueuedMessages(app *apps.App, event *pusher.WebhookEvent) {
	// loop through app webhooks
	// send to queue for each webhook and include the webhook
	for _, webhook := range app.Webhooks {
		if webhook.Filter.ChannelNameStartsWith != "" && !strings.HasPrefix(string(event.Channel), webhook.Filter.ChannelNameStartsWith) {
			continue
		}
		if webhook.Filter.ChannelNameEndsWith != "" && !strings.HasSuffix(string(event.Channel), webhook.Filter.ChannelNameEndsWith) {
			continue
		}
		data := &webhooks.QueuedJobData{
			Webhook:   &webhook,
			Payload:   event,
			AppKey:    app.Key,
			AppID:     app.ID,
			AppSecret: app.Secret,
		}
		q.concreteQueue.addToQueue(data)
	}
}

// sendWebhook is called by the queue implementation;
// this should happen after flap detection and any other middlewares
func (q *AbstractQueue) sendWebhook(data *webhooks.QueuedJobData) {

	evt := &pusher.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusher.WebhookEvent{*data.Payload},
	}
	q.webhookSender.Send(data, evt)
	// err := q.webhookManager.Send(evt)
	// if err != nil {
	// 	log.Logger().Errorf("Error sending webhook: %s", err)
	// }

}

// func (q *AbstractQueue) SendChannelCountChanges(app *apps.App, channelName constants.ChannelName, newCount int64, modifiedCount int64) {
// 	log.Logger().Tracef("Channel %s count changed to %d (modified by %d)", channelName, newCount, modifiedCount)
// 	if newCount == 0 {
// 		// publish a vacated channel event
// 		log.Logger().Debugf("Channel %s is now vacated", channelName)
// 		event := &pusher.WebhookEvent{
// 			Name:    string(constants.WebHookChannelVacated),
// 			Channel: channelName,
// 		}
// 		q.Send(app.ID, event)
// 	} else if newCount == modifiedCount {
// 		// if the count is equal to the amount we added, publish a channel occupied event
// 		log.Logger().Debugf("Channel %s is now occupied", channelName)
// 		event := &pusher.WebhookEvent{
// 			Name:    string(constants.WebHookChannelOccupied),
// 			Channel: channelName,
// 		}
// 		q.Send(app.ID, event)
// 	}
//
// 	// in addition to the above, we will also send a 'subscription_count' webhook
// 	// TODO: Cannot implement until pusher.WebhookEvent is updated to include SubscriptionCount
// 	// event := pusherClient.WebhookEvent{
// 	//	Name:    string(constants.WebHookSubscriptionCount),
// 	//	Channel: string(channelName),
// 	//	SubscriptionCount:
// 	// }
// 	// dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channelName), dispatcher.Connect, event)
// }

func (q *AbstractQueue) SendClientEvent(app *apps.App, channel constants.ChannelName, event string, data string, socketID constants.SocketID, userID constants.UserID) {
	if !app.HasClientEventWebhooks {
		return
	}

	eventData := &pusher.WebhookEvent{
		Name:     event,
		Channel:  channel,
		Data:     data,
		SocketID: socketID,
		UserID:   userID,
	}
	q.Send(app, eventData)
}

func (q *AbstractQueue) SendMemberAdded(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
	if !app.HasMemberAddedWebhooks {
		return
	}

	event := &pusher.WebhookEvent{
		Name:    string(constants.WebHookMemberAdded),
		Channel: channel,
		UserID:  userID,
	}
	q.Send(app, event)
}

func (q *AbstractQueue) SendMemberRemoved(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
	if !app.HasMemberRemovedWebhooks {
		return
	}

	event := &pusher.WebhookEvent{
		Name:    string(constants.WebHookMemberRemoved),
		Channel: channel,
		UserID:  userID,
	}
	q.Send(app, event)
}

func (q *AbstractQueue) SendChannelVacated(app *apps.App, channel constants.ChannelName) {
	if !app.HasChannelVacatedWebhooks {
		return
	}
	event := &pusher.WebhookEvent{
		Name:    string(constants.WebHookChannelVacated),
		Channel: channel,
	}
	q.Send(app, event)
}

func (q *AbstractQueue) SendChannelOccupied(app *apps.App, channel constants.ChannelName) {
	if !app.HasChannelOccupiedWebhooks {
		return
	}

	event := &pusher.WebhookEvent{
		Name:    string(constants.WebHookChannelOccupied),
		Channel: channel,
	}

	q.Send(app, event)
}

func (q *AbstractQueue) SendCacheMissed(app *apps.App, channel constants.ChannelName) {
	if !app.HasChannelOccupiedWebhooks {
		return
	}

	event := &pusher.WebhookEvent{
		Name:    string(constants.WebHookCacheMiss),
		Channel: channel,
	}
	q.Send(app, event)
}

func (q *AbstractQueue) sendWebhookByBatching(_ *apps.App, _ pusher.Webhook, _ string) {
	//
}
