package apps

import (
	"time"

	"pusher/internal/constants"
)

// will contain config params that are app-specific
// will contain helper functions like getting the signingTokenFromRequest

type App struct {
	ID                           constants.AppID
	Key                          string
	Secret                       string
	ActivityTimeout              int
	ReadTimeout                  time.Duration
	AuthenticationTimeout        time.Duration
	MaxConnections               int64
	EnableClientMessages         bool
	Enabled                      bool
	MaxBackendEventsPerSecond    int
	MaxClientEventsPerSecond     int
	MaxReadRequestsPerSecond     int
	Webhooks                     []constants.Webhook
	MaxPresenceMembersPerChannel int
	MaxPresenceMemberSizeInKb    int
	MaxChannelNameLength         int
	MaxEventChannelsAtOnce       int
	MaxEventNameLength           int
	MaxEventPayloadInKb          int
	MaxEventBatchSize            int
	EnableUserAuthentication     bool
	HasClientEventWebhooks       bool
	HasChannelOccupiedWebhooks   bool
	HasChannelVacatedWebhooks    bool
	HasMemberAddedWebhooks       bool
	HasMemberRemovedWebhooks     bool
	HasCacheMissWebhooks         bool
	WebhookBatchingEnabled       bool
	WebhooksEnabled              bool
}
