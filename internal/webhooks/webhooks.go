package webhooks

import (
	pusherClient "github.com/pusher/pusher-http-go/v5"
)

type WebhookContract interface {
	Send(webhook pusherClient.Webhook) error
}
