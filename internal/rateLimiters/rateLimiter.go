package rateLimiters

import (
	"pusher/internal"
	"pusher/internal/apps"
)

type RateLimiterInterface interface {
	Init() error
	ConsumeBackendEventPoints(points number, app *apps.App, ws *internal.WebSocket)
}
