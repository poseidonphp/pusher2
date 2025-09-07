package queues

import (
	"context"
	"sync"
	"time"

	"github.com/pusher/pusher-http-go/v5"
	"pusher/internal/apps"
	"pusher/log"
)

type EventType string

const (
	Connect    EventType = "connect"
	Disconnect EventType = "disconnect"
)

type pendingEvent struct {
	eventType EventType
	timer     *time.Timer
	cancel    context.CancelFunc
}

type FlapDetector struct {
	mu          sync.Mutex
	FlapEnabled bool                     // should we enable flap detection? If disabled, all events are sent.
	pending     map[string]*pendingEvent // key could be userID or userID+channel
	// dispatcher          QueueInterface
	flapWindowInSeconds time.Duration
}

func (fd *FlapDetector) Init() {
	if !fd.FlapEnabled {
		return
	}
	fd.pending = make(map[string]*pendingEvent)
	// default flap detection to 3 seconds if not set
	if fd.flapWindowInSeconds == 0*time.Second {
		fd.flapWindowInSeconds = 3 * time.Second
	}

}

func (fd *FlapDetector) CheckForFlapping(app *apps.App, key string, event EventType, webhookEvent *pusher.WebhookEvent, dispatchFn func(app *apps.App, webhookEvent *pusher.WebhookEvent)) {
	if !fd.FlapEnabled {
		dispatchFn(app, webhookEvent)
		return
	}
	log.Logger().Tracef("Received new event to handle: %s, %s [%s]", event, webhookEvent.Name, key)
	fd.mu.Lock()
	defer fd.mu.Unlock()

	opposite := Disconnect
	if event == Disconnect {
		opposite = Connect
	}

	if existing, ok := fd.pending[key]; ok && existing.eventType == opposite {
		// Flap detected, cancel both
		existing.cancel()
		existing.timer.Stop()
		delete(fd.pending, key)
		return
	}

	// No flap, so delay and fire
	ctx, cancel := context.WithCancel(context.Background())

	timer := time.AfterFunc(fd.flapWindowInSeconds, func() {
		select {
		case <-ctx.Done():
			log.Logger().Tracef("Event %s cancelled for key %s", event, key)
			return
		default:
			dispatchFn(app, webhookEvent) // send to dispatcher
		}
		fd.mu.Lock()
		delete(fd.pending, key)
		fd.mu.Unlock()
	})

	fd.pending[key] = &pendingEvent{
		eventType: event,
		timer:     timer,
		cancel:    cancel,
	}
}

// TODO: Implement this function to handle subscription count changes - BLOCKED until WebhookEvent is updated to include SubscriptionCount
// func (fd *FlapDetector) HandleSubscriptionCountChanges(channel constants.ChannelName, newCount int64, oldCount int64) {
//	if !env.GetBool("WEBHOOK_ENABLED", false) {
//		return
//	}
//	log.Logger().Tracef("Received new subscription count: %s (%d -> %d)", channel, oldCount, newCount)
//	fd.mu.Lock()
//	defer fd.mu.Unlock()
//
//	if newCount > oldCount {
//		fd.HandleEvent(string(channel), Connect, pusherClient.WebhookEvent{
//			Name:    "subscription_count",
//			Channel: string(channel),
//			Data:    map[string]interface{}{"count": newCount},
//		})
//	} else {
//		fd.HandleEvent(string(channel), Disconnect, pusherClient.WebhookEvent{
//			Name:    "subscription_count",
//			Channel: string(channel),
//			Data:    map[string]interface{}{"count": newCount},
//		})
//	}
// }
