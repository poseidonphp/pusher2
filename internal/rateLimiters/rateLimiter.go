package rateLimiters

import (
	"pusher/internal"
	"pusher/internal/apps"
)

type RateLimiterInterface interface {
	Init() error
	ConsumeBackendEventPoints(points int, app *apps.App, ws *internal.WebSocket)
}
