package dispatcher

import (
	"context"
	"sync"
	"time"
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
	mu                  sync.Mutex
	pending             map[string]*pendingEvent // key could be userID or userID+channel
	WebhookEnabled      bool
	dispatcher          DispatcherContract
	flapWindowInSeconds time.Duration
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
