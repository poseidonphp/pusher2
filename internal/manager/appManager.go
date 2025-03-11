package manager

type Contract interface {
	GetAllApps() []string
	GetApp(name string) (string, bool)
	GetAppSecret(name string) (ID string, key string, secret string, err error) // used when validating an API request
	GetAppAndKey(name string) (ID string, key string, err error)                // used when establishing a new socket
}
