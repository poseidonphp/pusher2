package dispatcher

import (
	"context"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/env"
	"pusher/log"
	"sync"
	"time"
)

type EventType string

const (
	Connect    EventType = "connect"
	Disconnect EventType = "disconnect"
)

var DispatchBuffer *FlapDetector

type pendingEvent struct {
	eventType EventType
	timer     *time.Timer
	cancel    context.CancelFunc
}

type FlapDetector struct {
	mu      sync.Mutex
	pending map[string]*pendingEvent // key could be userID or userID+channel
}

func InitFlapDetector() {
	log.Logger().Infoln("Initializing the flap detector")
	DispatchBuffer = &FlapDetector{
		pending: make(map[string]*pendingEvent),
	}
}

// TODO: Implement this function to handle subscription count changes - BLOCKED until WebhookEvent is updated to include SubscriptionCount
//func (fd *FlapDetector) HandleSubscriptionCountChanges(channel constants.ChannelName, newCount int64, oldCount int64) {
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
//}

func (fd *FlapDetector) HandleEvent(key string, event EventType, webhookEvent pusherClient.WebhookEvent) {
	// TODO: handle frequent updates such as multiple clients connecting - if count is > 100, throttle to only send every 5 seconds
	// https://pusher.com/docs/channels/server_api/webhooks/#subscription-count-events
	if !env.GetBool("WEBHOOK_ENABLED", false) {
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
	timer := time.AfterFunc(3*time.Second, func() {
		select {
		case <-ctx.Done():
			log.Logger().Tracef("Event %s cancelled for key %s", event, key)
			return // cancelled
		default:
			Dispatcher.Dispatch(webhookEvent) // send to dispatcher
			//webhookFunc(webhookEvent) // safe to call external webhook
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
