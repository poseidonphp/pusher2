package queues

import (
	"context"
	"time"

	"pusher/internal/constants"
	"pusher/internal/util"
	"pusher/internal/webhooks"
	"pusher/log"
)

func NewSyncQueue(ctx context.Context, webhookSender *webhooks.WebhookSender) (*SyncQueue, error) {
	s := &SyncQueue{}
	aq, err := NewAbstractQueue(ctx, s, webhookSender, true, 3*time.Second)
	if err != nil {
		log.Logger().Errorf("Error creating AbstractQueue: %v", err)
		return nil, err
	}
	s.AbstractQueue = aq
	return s, nil
}

// This is a local dispatcher - it will dispatch all messages in realtime

type SyncQueue struct {
	// QueueCore
	*AbstractQueue
	incomingMessages chan *webhooks.QueuedJobData
	// WebhookManager   webhooks.WebhookInterface
}

func (sd *SyncQueue) Init() error {
	sd.incomingMessages = make(chan *webhooks.QueuedJobData, 100)
	return nil
}

func (sd *SyncQueue) addToQueue(jobData *webhooks.QueuedJobData) {
	// put the server event in the channel
	log.Logger().Tracef("Received event to dispatch: %s (%s)", jobData.Payload.Name, jobData.Payload.Channel)
	if util.IsPrivateEncryptedChannel(constants.ChannelName(jobData.Payload.Channel)) {
		log.Logger().Tracef("Event is for private encrypted channel: %s", jobData.Payload.Channel)
		// need to encrypt the data parameter
		// TODO need to add this to dispatchRedis, or extract to a common function
		// TODO how do i encrypt the data if i don't have the encryption key?
	}
	sd.incomingMessages <- jobData
}

func (sd *SyncQueue) monitorQueue(ctx context.Context) {
	log.Logger().Infoln("Listening for events on local dispatcher")
	for {
		select {
		case data := <-sd.incomingMessages:
			// process the server event
			log.Logger().Debugf("Dispatching event: %s (%v)", data.Payload.Name, data.Payload.Channel)
			// wh := &pusher.Webhook{
			// 	TimeMs: int(time.Now().UnixMilli()),
			// 	Events: []pusher.WebhookEvent{*data},
			// }

			// sErr := sd.WebhookManager.Send(*wh)
			sd.sendWebhook(data)
			// if sErr != nil {
			// 	log.Logger().Errorf("Error sending webhook: %s", sErr)
			// }
		case <-ctx.Done():
			log.Logger().Debugln("Stopping local dispatcher due to context cancellation")
			return
		}
	}
}
