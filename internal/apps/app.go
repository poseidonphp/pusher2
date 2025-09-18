package apps

import (
	"math"
	"time"

	"pusher/internal/constants"
)

type App struct {
	// ID should be a string of numbers only. It is used by the client server to connect to the pusher server
	ID constants.AppID `mapstructure:"app_id"`

	// Key is used by the end user client to identify the app they want to connect to
	Key string `mapstructure:"app_key"`

	// Secret is used by the client server to connect to the pusher server
	Secret string `mapstructure:"app_secret"`

	// ActivityTimeout is in seconds is how much time the server will tell the client to wait between pings
	ActivityTimeout int `mapstructure:"app_activity_timeout"`

	// ReadTimeout is calculated based on ActivityTimeout to allow for some network latency.
	// It is the time the server will wait before closing an idle connection.
	ReadTimeout time.Duration `mapstructure:"-" json:"-"`

	// AuthorizationTimeoutSeconds is in seconds, how long the server will wait for a client to authenticate (if requested)
	// before closing the connection
	AuthorizationTimeoutSeconds int           `mapstructure:"app_authorization_timeout_seconds"`
	AuthorizationTimeout        time.Duration `mapstructure:"-" json:"-"`

	// MaxConnections is the maximum number of concurrent connections this app is allowed to have.
	// -1 means unlimited
	MaxConnections int64 `mapstructure:"app_max_connections"`

	// EnableClientMessages allows the client to send messages to the server or other clients via the server
	EnableClientMessages bool `mapstructure:"app_enable_client_messages"`

	// Enabled determines if the app is enabled or disabled. Disabled apps will reject all connections
	Enabled bool `mapstructure:"app_enabled"`

	// MaxBackendEventsPerSecond is the maximum number of events per second the app is allowed to send via the server SDKs or REST API
	MaxBackendEventsPerSecond int                 `mapstructure:"app_max_backend_events_per_second"`
	MaxClientEventsPerSecond  int                 `mapstructure:"app_max_client_events_per_second"`
	MaxReadRequestsPerSecond  int                 `mapstructure:"app_max_read_requests_per_second"`
	Webhooks                  []constants.Webhook `mapstructure:"-" json:"-"`

	// MaxPresenceMembersPerChannel is the maximum number of presence members allowed in a presence channel
	MaxPresenceMembersPerChannel int `mapstructure:"app_max_presence_members_per_channel"`

	// MaxPresenceMemberSizeInKb is the maximum size of the member info JSON object in kilobytes
	MaxPresenceMemberSizeInKb int `mapstructure:"app_max_presence_member_size_in_kb"`

	// MaxChannelNameLength is the maximum length of a channel name
	MaxChannelNameLength int `mapstructure:"app_max_channel_name_length"`

	// MaxEventChannelsAtOnce is the maximum number of channels that can be specified when sending an event
	MaxEventChannelsAtOnce int `mapstructure:"app_max_event_channels_at_once"`
	MaxEventNameLength     int `mapstructure:"app_max_event_name_length"`

	// MaxEventPayloadInKb is the maximum size of the event data payload in kilobytes
	MaxEventPayloadInKb int `mapstructure:"app_max_event_payload_in_kb"`
	MaxEventBatchSize   int `mapstructure:"app_max_event_batch_size"`

	// RequireChannelAuthorization forces the client to authenticate when establishing a connection to the pusher server,
	// regardless of the channel type they are subscribing to. This is useful for ensuring that only authorized clients can connect to the server.
	// If this is enabled, the client must authenticate within the AuthorizationTimeoutSeconds period after connecting,
	// or the server will close the connection. This is also required to be able to terminate all connections for
	// a specific user across all devices via the REST API.
	//
	// Note: This does not replace the need to authenticate for private and presence channels, which is still required.
	RequireChannelAuthorization bool   `mapstructure:"app_require_channel_authorization"`
	HasClientEventWebhooks      bool   `mapstructure:"app_has_client_event_webhooks"`
	HasChannelOccupiedWebhooks  bool   `mapstructure:"app_has_channel_occupied_webhooks"`
	HasChannelVacatedWebhooks   bool   `mapstructure:"app_has_channel_vacated_webhooks"`
	HasMemberAddedWebhooks      bool   `mapstructure:"app_has_member_added_webhooks"`
	HasMemberRemovedWebhooks    bool   `mapstructure:"app_has_member_removed_webhooks"`
	HasCacheMissWebhooks        bool   `mapstructure:"app_has_cache_miss_webhooks"`
	WebhookBatchingEnabled      bool   `mapstructure:"app_webhook_batching_enabled"`
	WebhooksEnabled             bool   `mapstructure:"app_webhooks_enabled"`
	WebhookURL                  string `mapstructure:"app_webhook_url"`
	WebhookSNSRegion            string `mapstructure:"app_webhook_sns_region"`
	WebhookSNSTopicARN          string `mapstructure:"app_webhook_sns_topic_arn"`
	WebhookFilterPrefix         string `mapstructure:"app_webhook_filter_prefix"`
}

func (a *App) SetMissingDefaults() {
	a.ActivityTimeout = getValueOrFallback(a.ActivityTimeout, 60)

	// We set the ReadTimeout here, as it is a calculated value based on ActivityTimeout
	activityTimeoutInt := int64(math.Round((float64(a.ActivityTimeout) * 10.0) / 9.0))
	a.ReadTimeout = time.Duration(activityTimeoutInt) * time.Second

	// We set the AuthorizationTimeoutSeconds default first, then calculate the AuthorizationTimeout
	a.AuthorizationTimeoutSeconds = getValueOrFallback(a.AuthorizationTimeoutSeconds, 5)
	a.AuthorizationTimeout = time.Duration(a.AuthorizationTimeoutSeconds) * time.Second

	a.MaxConnections = getValueOrFallback(a.MaxConnections, -1)
	a.EnableClientMessages = getValueOrFallback(a.EnableClientMessages, false)
	a.Enabled = getValueOrFallback(a.Enabled, true)
	a.MaxBackendEventsPerSecond = getValueOrFallback(a.MaxBackendEventsPerSecond, -1)
	a.MaxClientEventsPerSecond = getValueOrFallback(a.MaxClientEventsPerSecond, -1)
	a.MaxReadRequestsPerSecond = getValueOrFallback(a.MaxReadRequestsPerSecond, -1)
	a.MaxPresenceMembersPerChannel = getValueOrFallback(a.MaxPresenceMembersPerChannel, 100)
	a.MaxPresenceMemberSizeInKb = getValueOrFallback(a.MaxPresenceMemberSizeInKb, 10)
	a.MaxChannelNameLength = getValueOrFallback(a.MaxChannelNameLength, 100)
	a.MaxEventChannelsAtOnce = getValueOrFallback(a.MaxEventChannelsAtOnce, 10)
	a.MaxEventNameLength = getValueOrFallback(a.MaxEventNameLength, 100)
	a.MaxEventPayloadInKb = getValueOrFallback(a.MaxEventPayloadInKb, 100)
	a.MaxEventBatchSize = getValueOrFallback(a.MaxEventBatchSize, 20)
	a.RequireChannelAuthorization = getValueOrFallback(a.RequireChannelAuthorization, false)
	a.HasClientEventWebhooks = getValueOrFallback(a.HasClientEventWebhooks, false)
	a.HasChannelOccupiedWebhooks = getValueOrFallback(a.HasChannelOccupiedWebhooks, false)
	a.HasChannelVacatedWebhooks = getValueOrFallback(a.HasChannelVacatedWebhooks, false)
	a.HasMemberAddedWebhooks = getValueOrFallback(a.HasMemberAddedWebhooks, false)
	a.HasMemberRemovedWebhooks = getValueOrFallback(a.HasMemberRemovedWebhooks, false)
	a.HasCacheMissWebhooks = getValueOrFallback(a.HasCacheMissWebhooks, false)
	a.WebhookBatchingEnabled = getValueOrFallback(a.WebhookBatchingEnabled, false)
	a.WebhooksEnabled = getValueOrFallback(a.WebhooksEnabled, false)

	if a.Webhooks == nil {
		a.Webhooks = make([]constants.Webhook, 0, len(a.Webhooks))
	}
}

func getValueOrFallback[T string | int | int64 | bool | time.Duration](value T, fallback T) T {
	switch v := any(value).(type) {
	case string:
		if v == "" {
			return fallback
		}
	case int:
		if v == 0 {
			return fallback
		}
	case int64:
		if v == 0 {
			return fallback
		}
	case bool:
		if !v {
			return fallback
		}
	case time.Duration:
		if v == 0 {
			return fallback
		}
	}
	return value
}
