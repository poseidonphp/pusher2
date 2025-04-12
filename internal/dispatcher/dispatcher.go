package dispatcher

import (
	"context"
	"github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/log"
	"time"
)

type DispatcherContract interface {
	Dispatch(webhookEvent pusher.WebhookEvent)
	ListenForEvents(ctx context.Context)
	Init() error
	InitFlapDetection(webhookEnabled bool, dispatchManager DispatcherContract, flapWindowInSeconds int)
	DispatchFlap(key string, event EventType, webhookEvent pusher.WebhookEvent)
	SendChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64)
}

type DispatcherCore struct {
	FlapDetector *FlapDetector
}

func (dc *DispatcherCore) InitFlapDetection(webhookEnabled bool, dispatchManager DispatcherContract, flapWindowInSeconds int) {
	log.Logger().Debugln("Initializing the flap detector")
	dc.FlapDetector = &FlapDetector{
		pending:             make(map[string]*pendingEvent),
		WebhookEnabled:      webhookEnabled,
		dispatcher:          dispatchManager,
		flapWindowInSeconds: time.Duration(flapWindowInSeconds) * time.Second,
	}
}

func (dc *DispatcherCore) DispatchFlap(key string, event EventType, webhookEvent pusher.WebhookEvent) {
	// TODO: handle frequent updates such as multiple clients connecting - if count is > 100, throttle to only send every 5 seconds
	// https://pusher.com/docs/channels/server_api/webhooks/#subscription-count-events
	if !dc.FlapDetector.WebhookEnabled {
		return
	}
	log.Logger().Tracef("Received new event to handle: %s, %s [%s]", event, webhookEvent.Name, key)
	dc.FlapDetector.mu.Lock()
	defer dc.FlapDetector.mu.Unlock()

	opposite := Disconnect
	if event == Disconnect {
		opposite = Connect
	}

	if existing, ok := dc.FlapDetector.pending[key]; ok && existing.eventType == opposite {
		// Flap detected, cancel both
		existing.cancel()
		existing.timer.Stop()
		delete(dc.FlapDetector.pending, key)
		return
	}

	// No flap, so delay and fire
	ctx, cancel := context.WithCancel(context.Background())

	timer := time.AfterFunc(dc.FlapDetector.flapWindowInSeconds, func() {
		select {
		case <-ctx.Done():
			log.Logger().Tracef("Event %s cancelled for key %s", event, key)
			return
		default:
			dc.FlapDetector.dispatcher.Dispatch(webhookEvent) // send to dispatcher
		}

		dc.FlapDetector.mu.Lock()
		delete(dc.FlapDetector.pending, key)
		dc.FlapDetector.mu.Unlock()
	})

	dc.FlapDetector.pending[key] = &pendingEvent{
		eventType: event,
		timer:     timer,
		cancel:    cancel,
	}
}

func (dc *DispatcherCore) SendChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64) {
	if dc.FlapDetector == nil {
		log.Logger().Errorf("Flap detector is not initialized")
		return
	}
	log.Logger().Tracef("Channel %s count changed to %d (modified by %d)", channelName, newCount, modifiedCount)
	if newCount == 0 {
		// publish a vacated channel event
		log.Logger().Debugf("Channel %s is now vacated", channelName)
		event := pusher.WebhookEvent{
			Name:    string(constants.WebHookChannelVacated),
			Channel: string(channelName),
		}
		dc.DispatchFlap("channel:"+string(channelName), Disconnect, event)
	} else if newCount == modifiedCount {
		// if the count is equal to the amount we added, publish a channel occupied event
		log.Logger().Debugf("Channel %s is now occupied", channelName)
		event := pusher.WebhookEvent{
			Name:    string(constants.WebHookChannelOccupied),
			Channel: string(channelName),
		}
		dc.DispatchFlap("channel:"+string(channelName), Connect, event)
	}

	// in addition to the above, we will also send a 'subscription_count' webhook
	// TODO: Cannot implement until WebhookEvent is updated to include SubscriptionCount
	//event := pusherClient.WebhookEvent{
	//	Name:    string(constants.WebHookSubscriptionCount),
	//	Channel: string(channelName),
	//	SubscriptionCount:
	//}
	//dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channelName), dispatcher.Connect, event)
}
