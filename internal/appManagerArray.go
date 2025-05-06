package internal

import (
	"errors"
	"sync"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/log"
)

type ArrayAppManager struct {
	Apps  map[constants.AppID]apps.App
	mutex sync.Mutex
}

func (a *ArrayAppManager) Init(appsFromConfig []apps.App) error {
	a.Apps = make(map[constants.AppID]apps.App)
	for _, app := range appsFromConfig {
		a.Apps[app.ID] = app
	}

	if len(a.Apps) == 0 {
		log.Logger().Errorf("No apps found. Please provide a config file or set environment variables.")
		return errors.New("no apps found")
	}

	return nil
}

// func (a *ArrayAppManager) loadAppFromEnv() error {
//
// 	var configuredWebhooks []constants.Webhook
// 	if env.GetBool("WEBHOOK_ENABLED", false) {
// 		if env.GetString("WEBHOOK_URL", "") != "" {
// 			webhook := constants.Webhook{
// 				URL: env.GetString("WEBHOOK_URL"),
// 				Filter: constants.WebhookFilters{
// 					ChannelNameStartsWith: env.GetString("WEBHOOK_FILTER_PREFIX"),
// 					ChannelNameEndsWith:   env.GetString("WEBHOOK_FILTER_SUFFIX"),
// 				},
// 			}
// 			configuredWebhooks = append(configuredWebhooks, webhook)
// 		}
//
// 		if env.GetString("WEBHOOK_SNS_TOPIC_ARN", "") != "" && env.GetString("WEBHOOK_SNS_REGION", "") != "" {
// 			webhook := constants.Webhook{
// 				SNSRegion:   env.GetString("WEBHOOK_SNS_REGION"),
// 				SNSTopicARN: env.GetString("WEBHOOK_SNS_TOPIC_ARN"),
// 				Filter: constants.WebhookFilters{
// 					ChannelNameStartsWith: env.GetString("WEBHOOK_FILTER_PREFIX"),
// 					ChannelNameEndsWith:   env.GetString("WEBHOOK_FILTER_SUFFIX"),
// 				},
// 			}
// 			configuredWebhooks = append(configuredWebhooks, webhook)
// 		}
// 	}
//
// }

func (a *ArrayAppManager) GetAllApps() []apps.App {
	applications := make([]apps.App, 0, len(a.Apps))
	for _, app := range a.Apps {
		applications = append(applications, app)
	}
	return applications
}

func (a *ArrayAppManager) GetAppSecret(id constants.AppID) (string, error) {
	if app, ok := a.Apps[id]; ok {
		return app.Secret, nil
	} else {
		return "", errors.New("app not found")
	}
}

func (a *ArrayAppManager) FindByID(id constants.AppID) (*apps.App, error) {
	if app, ok := a.Apps[id]; ok {
		return &app, nil
	} else {
		return nil, errors.New("app not found")
	}
}

func (a *ArrayAppManager) FindByKey(key string) (*apps.App, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, app := range a.Apps {
		if app.Key == key {
			return &app, nil
		}
	}
	return nil, errors.New("app not found")
}
