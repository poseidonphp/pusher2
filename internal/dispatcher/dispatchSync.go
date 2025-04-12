package dispatcher

import (
	"context"
	"github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/internal/util"
	"pusher/internal/webhooks"
	"pusher/log"
	"time"
)

// This is a local dispatcher - it will dispatch all messages in realtime

type SyncDispatcher struct {
	DispatcherCore
	incomingMessages chan pusher.WebhookEvent
	WebhookManager   webhooks.WebhookContract
}

func (sd *SyncDispatcher) Init() error {
	sd.incomingMessages = make(chan pusher.WebhookEvent, 100)
	return nil
}

func (sd *SyncDispatcher) Dispatch(serverEvent pusher.WebhookEvent) {
	// put the server event in the channel
	log.Logger().Tracef("Received event to dispatch: %s (%s)", serverEvent.Name, serverEvent.Channel)
	if util.IsPrivateEncryptedChannel(constants.ChannelName(serverEvent.Channel)) {
		log.Logger().Tracef("Event is for private encrypted channel: %s", serverEvent.Channel)
		// need to encrypt the data parameter
		// TODO need to add this to dispatchRedis, or extract to a common function
		// TODO how do i encrypt the data if i don't have the encryption key?
	}
	sd.incomingMessages <- serverEvent
}

func (sd *SyncDispatcher) ListenForEvents(ctx context.Context) {
	log.Logger().Infoln("Listening for events on local dispatcher")
	for {
		select {
		case serverEvent := <-sd.incomingMessages:
			// process the server event
			log.Logger().Debugf("Dispatching event: %s (%v)", serverEvent.Name, serverEvent.Channel)
			wh := &pusher.Webhook{
				TimeMs: int(time.Now().UnixMilli()),
				Events: []pusher.WebhookEvent{serverEvent},
			}

			sErr := sd.WebhookManager.Send(*wh)
			if sErr != nil {
				log.Logger().Errorf("Error sending webhook: %s", sErr)
			}
		case <-ctx.Done():
			log.Logger().Debugln("Stopping local dispatcher due to context cancellation")
			return
		}
	}
}
