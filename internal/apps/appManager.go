package apps

import (
	"pusher/internal/constants"
)

type AppManagerContract interface {
	Init() error
	GetAllApps() []App
	GetAppSecret(id constants.AppID) (string, error) // used when validating an API request
	FindByID(id constants.AppID) (*App, error)
	FindByKey(key string) (*App, error)
}
