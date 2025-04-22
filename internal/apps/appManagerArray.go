package apps

import (
	"errors"
	"sync"
	"time"

	"pusher/env"
	"pusher/internal/constants"
)

type ArrayAppManager struct {
	Apps  map[constants.AppID]App
	mutex sync.Mutex
}

func (a *ArrayAppManager) Init() error {
	a.Apps = make(map[constants.AppID]App)
	if env.GetString("APP_ID") == "" {
		return errors.New("APP_ID is not set")
	}
	if env.GetString("APP_KEY") == "" {
		return errors.New("APP_KEY is not set")
	}
	if env.GetString("APP_SECRET") == "" {
		return errors.New("APP_SECRET is not set")
	}
	appId := constants.AppID(env.GetString("APP_ID"))

	var configuredWebhooks []constants.Webhook
	if env.GetBool("WEBHOOK_ENABLED", false) {
		if env.GetString("WEBHOOK_URL", "") != "" {
			webhook := constants.Webhook{
				URL: env.GetString("WEBHOOK_URL"),
				Filter: constants.WebhookFilters{
					ChannelNameStartsWith: env.GetString("WEBHOOK_FILTER_PREFIX"),
					ChannelNameEndsWith:   env.GetString("WEBHOOK_FILTER_SUFFIX"),
				},
			}
			configuredWebhooks = append(configuredWebhooks, webhook)
		}

		if env.GetString("WEBHOOK_SNS_TOPIC_ARN", "") != "" && env.GetString("WEBHOOK_SNS_REGION", "") != "" {
			webhook := constants.Webhook{
				SNSRegion:   env.GetString("WEBHOOK_SNS_REGION"),
				SNSTopicARN: env.GetString("WEBHOOK_SNS_TOPIC_ARN"),
				Filter: constants.WebhookFilters{
					ChannelNameStartsWith: env.GetString("WEBHOOK_FILTER_PREFIX"),
					ChannelNameEndsWith:   env.GetString("WEBHOOK_FILTER_SUFFIX"),
				},
			}
			configuredWebhooks = append(configuredWebhooks, webhook)
		}
	}

	activityTimeout := env.GetInt("ACTIVITY_TIMEOUT", 60)

	a.Apps[appId] = App{
		ID:                           appId,
		Key:                          env.GetString("APP_KEY"),
		Secret:                       env.GetString("APP_SECRET"),
		ActivityTimeout:              activityTimeout,
		ReadTimeout:                  time.Duration(float64(activityTimeout)*10.0/9.0) * time.Second,
		AuthenticationTimeout:        env.GetDuration("AUTHENTICATION_TIMEOUT", 5) * time.Second,
		MaxConnections:               env.GetInt64("MAX_CONNECTIONS", 1000),
		EnableClientMessages:         env.GetBool("ENABLE_CLIENT_MESSAGES", false),
		Enabled:                      true,
		MaxBackendEventsPerSecond:    0,
		MaxClientEventsPerSecond:     0,
		MaxReadRequestsPerSecond:     0,
		Webhooks:                     configuredWebhooks,
		MaxPresenceMembersPerChannel: env.GetInt("MAX_PRESENCE_USERS", 100),
		MaxPresenceMemberSizeInKb:    env.GetInt("MAX_PRESENCE_USER_DATA_KB", 10),
		MaxChannelNameLength:         env.GetInt("MAX_CHANNEL_NAME_LENGTH", 100),
		MaxEventChannelsAtOnce:       env.GetInt("MAX_EVENT_CHANNELS_AT_ONCE", 10),
		MaxEventNameLength:           env.GetInt("MAX_EVENT_NAME_LENGTH", 100),
		MaxEventPayloadInKb:          env.GetInt("MAX_EVENT_PAYLOAD_KB", 100),
		MaxEventBatchSize:            env.GetInt("MAX_EVENT_BATCH_SIZE", 20),
		EnableUserAuthentication:     env.GetBool("ENABLE_USER_AUTHENTICATION", false),
		HasClientEventWebhooks:       env.GetBool("WEBHOOK_EVENT_CLIENT_EVENTS", true),
		HasChannelOccupiedWebhooks:   env.GetBool("WEBHOOK_EVENT_CHANNEL_OCCUPIED", true),
		HasChannelVacatedWebhooks:    env.GetBool("WEBHOOK_EVENT_CHANNEL_VACATED", true),
		HasMemberAddedWebhooks:       env.GetBool("WEBHOOK_EVENT_MEMBER_ADDED", true),
		HasMemberRemovedWebhooks:     env.GetBool("WEBHOOK_EVENT_MEMBER_REMOVED", true),
		HasCacheMissWebhooks:         env.GetBool("WEBHOOK_EVENT_CACHE_MISS", true),
		WebhookBatchingEnabled:       env.GetBool("WEBHOOK_EVENT_BATCHING", true),
		WebhooksEnabled:              env.GetBool("WEBHOOK_ENABLED", false),
	}

	return nil
}

func (a *ArrayAppManager) GetAllApps() []App {
	apps := make([]App, 0, len(a.Apps))
	for _, app := range a.Apps {
		apps = append(apps, app)
	}
	return apps
}

func (a *ArrayAppManager) GetAppSecret(id constants.AppID) (string, error) {
	if app, ok := a.Apps[id]; ok {
		return app.Secret, nil
	} else {
		return "", errors.New("app not found")
	}
}

func (a *ArrayAppManager) FindByID(id constants.AppID) (*App, error) {
	if app, ok := a.Apps[id]; ok {
		return &app, nil
	} else {
		return nil, errors.New("app not found")
	}
}

func (a *ArrayAppManager) FindByKey(key string) (*App, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, app := range a.Apps {
		if app.Key == key {
			return &app, nil
		}
	}
	return nil, errors.New("app not found")
}
