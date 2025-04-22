package cache

import (
	"fmt"
	"time"

	"pusher/internal/constants"
)

type CacheContract interface {
	Init() error
	Get(key string) (string, bool)
	Set(key string, value string)                      // create or update the key with the value
	SetEx(key string, value string, ttl time.Duration) // create or update the key with the value and set expiration
	Delete(key string)
	Remember(key string, ttl int, callback func() (string, error)) (string, error)
	Has(key string) bool
	Update(key string, value string) // Update the value only if it exists
}

func GetChannelCacheKey(appID constants.AppID, channelName string) string {
	return fmt.Sprintf("app#%s#channel#%s", appID, channelName)
}
