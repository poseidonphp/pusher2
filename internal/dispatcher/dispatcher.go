package dispatcher

import (
	"context"
	"github.com/pusher/pusher-http-go/v5"
)

var Dispatcher DispatcherContract

type DispatcherContract interface {
	Dispatch(webhookEvent pusher.WebhookEvent)
	ListenForEvents(ctx context.Context)
	Init()
}
