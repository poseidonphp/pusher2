package apps

import (
	"time"

	"pusher/internal/constants"
)

// will contain config params that are app-specific
// will contain helper functions like getting the signingTokenFromRequest

// type App struct {
// 	ID                           constants.AppID     ` json:"id"`
// 	Key                          string              `json:"key"`
// 	Secret                       string              `json:"secret"`
// 	ActivityTimeout              int                 `json:"activity_timeout"`
// 	ReadTimeout                  time.Duration       `json:"read_timeout"`
// 	AuthorizationTimeout         time.Duration       `json:"authorization_timeout"`
// 	MaxConnections               int64               `json:"max_connections"`
// 	EnableClientMessages         bool                `json:"enable_client_messages"`
// 	Enabled                      bool                `json:"enabled"`
// 	MaxBackendEventsPerSecond    int                 `json:"max_backend_events_per_second"`
// 	MaxClientEventsPerSecond     int                 `json:"max_client_events_per_second"`
// 	MaxReadRequestsPerSecond     int                 `json:"max_read_requests_per_second"`
// 	Webhooks                     []constants.Webhook `json:"webhooks"`
// 	MaxPresenceMembersPerChannel int                 `json:"max_presence_members_per_channel"`
// 	MaxPresenceMemberSizeInKb    int                 `json:"max_presence_member_size_in_kb"`
// 	MaxChannelNameLength         int                 `json:"max_channel_name_length"`
// 	MaxEventChannelsAtOnce       int                 `json:"max_event_channels_at_once"`
// 	MaxEventNameLength           int                 `json:"max_event_name_length"`
// 	MaxEventPayloadInKb          int                 `json:"max_event_payload_in_kb"`
// 	MaxEventBatchSize            int                 `json:"max_event_batch_size"`
// 	RequireChannelAuthorization  bool                `json:"require_channel_authorization"`
// 	HasClientEventWebhooks       bool                `json:"has_client_event_webhooks"`
// 	HasChannelOccupiedWebhooks   bool                `json:"has_channel_occupied_webhooks"`
// 	HasChannelVacatedWebhooks    bool                `json:"has_channel_vacated_webhooks"`
// 	HasMemberAddedWebhooks       bool                `json:"has_member_added_webhooks"`
// 	HasMemberRemovedWebhooks     bool                `json:"has_member_removed_webhooks"`
// 	HasCacheMissWebhooks         bool                `json:"has_cache_miss_webhooks"`
// 	WebhookBatchingEnabled       bool                `json:"webhook_batching_enabled"`
// 	WebhooksEnabled              bool                `json:"webhooks_enabled"`
// }

type App struct {
	ID                           constants.AppID     `mapstructure:"app_id"`
	Key                          string              `mapstructure:"app_key"`
	Secret                       string              `mapstructure:"app_secret"`
	ActivityTimeout              int                 `mapstructure:"app_activity_timeout"`
	ReadTimeout                  time.Duration       `mapstructure:"-" json:"-"`
	AuthorizationTimeoutSeconds  int                 `mapstructure:"app_authorization_timeout_seconds"`
	AuthorizationTimeout         time.Duration       `mapstructure:"-" json:"-"`
	MaxConnections               int64               `mapstructure:"app_max_connections"`
	EnableClientMessages         bool                `mapstructure:"app_enable_client_messages"`
	Enabled                      bool                `mapstructure:"app_enabled"`
	MaxBackendEventsPerSecond    int                 `mapstructure:"app_max_backend_events_per_second"`
	MaxClientEventsPerSecond     int                 `mapstructure:"app_max_client_events_per_second"`
	MaxReadRequestsPerSecond     int                 `mapstructure:"app_max_read_requests_per_second"`
	Webhooks                     []constants.Webhook `mapstructure:"-" json:"-"`
	MaxPresenceMembersPerChannel int                 `mapstructure:"app_max_presence_members_per_channel"`
	MaxPresenceMemberSizeInKb    int                 `mapstructure:"app_max_presence_member_size_in_kb"`
	MaxChannelNameLength         int                 `mapstructure:"app_max_channel_name_length"`
	MaxEventChannelsAtOnce       int                 `mapstructure:"app_max_event_channels_at_once"`
	MaxEventNameLength           int                 `mapstructure:"app_max_event_name_length"`
	MaxEventPayloadInKb          int                 `mapstructure:"app_max_event_payload_in_kb"`
	MaxEventBatchSize            int                 `mapstructure:"app_max_event_batch_size"`
	RequireChannelAuthorization  bool                `mapstructure:"app_require_channel_authorization"`
	HasClientEventWebhooks       bool                `mapstructure:"app_has_client_event_webhooks"`
	HasChannelOccupiedWebhooks   bool                `mapstructure:"app_has_channel_occupied_webhooks"`
	HasChannelVacatedWebhooks    bool                `mapstructure:"app_has_channel_vacated_webhooks"`
	HasMemberAddedWebhooks       bool                `mapstructure:"app_has_member_added_webhooks"`
	HasMemberRemovedWebhooks     bool                `mapstructure:"app_has_member_removed_webhooks"`
	HasCacheMissWebhooks         bool                `mapstructure:"app_has_cache_miss_webhooks"`
	WebhookBatchingEnabled       bool                `mapstructure:"app_webhook_batching_enabled"`
	WebhooksEnabled              bool                `mapstructure:"app_webhooks_enabled"`
}

func (a *App) SetMissingDefaults() {
	a.ActivityTimeout = getValueOrFallback(a.ActivityTimeout, 60)
	a.ReadTimeout = time.Duration(float64(a.ActivityTimeout)*10.0/9.0) * time.Second
	a.AuthorizationTimeout = getValueOrFallback(a.AuthorizationTimeout, 5) * time.Second
	a.MaxConnections = getValueOrFallback(a.MaxConnections, 0)
	a.EnableClientMessages = getValueOrFallback(a.EnableClientMessages, false)
	a.Enabled = getValueOrFallback(a.Enabled, true)
	a.MaxBackendEventsPerSecond = getValueOrFallback(a.MaxBackendEventsPerSecond, 0)
	a.MaxClientEventsPerSecond = getValueOrFallback(a.MaxClientEventsPerSecond, 0)
	a.MaxReadRequestsPerSecond = getValueOrFallback(a.MaxReadRequestsPerSecond, 0)
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
