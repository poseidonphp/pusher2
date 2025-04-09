package clients

import (
	"crypto/tls"
	"errors"
	"github.com/go-redis/redis"
	"pusher/env"
	"pusher/log"
	"strings"
	"sync"
)

type RedisClient struct {
	client redis.UniversalClient
	Prefix string
}

var RedisClientInstance *RedisClient

var redisOnce sync.Once

func (r *RedisClient) GetKey(key string) string {
	return r.Prefix + ":" + key
}

func (r *RedisClient) GetClient() redis.UniversalClient {
	if r.client == nil {
		log.Logger().Errorln("Redis client is not initialized")
	}
	return r.client
}

func (r *RedisClient) initRedisClient(redisURL string) error {
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		return err
	}
	universalOptions := &redis.UniversalOptions{
		Addrs:       []string{redisOptions.Addr},
		DB:          redisOptions.DB,
		Password:    redisOptions.Password,
		PoolSize:    redisOptions.PoolSize,
		PoolTimeout: redisOptions.PoolTimeout,
	}
	if env.GetBool("USE_TLS", false) {
		universalOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
	r.client = redis.NewUniversalClient(universalOptions)

	_, err = r.client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) initClusterRedisClient(redisURLs []string) error {
	var err error
	var nodes []string
	var redisOptions *redis.Options
	for _, redisURL := range redisURLs {
		redisOptions, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Logger().Panic(err)
		}
		nodes = append(nodes, redisOptions.Addr)
	}
	if redisOptions == nil {
		return errors.New("failed to initialize redis options")
	}
	universalOptions := &redis.UniversalOptions{
		Addrs:       nodes,
		DB:          redisOptions.DB,
		Password:    redisOptions.Password,
		PoolSize:    redisOptions.PoolSize,
		PoolTimeout: redisOptions.PoolTimeout,
	}
	if env.GetBool("USE_TLS", false) {
		universalOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
	r.client = redis.NewUniversalClient(universalOptions)

	_, err = r.client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) InitRedis(prefix string) error {
	if r.client == nil {
		redisOnce.Do(func() {
			redisURLs := strings.Split(env.GetString("REDIS_URL", "redis://localhost:6379"), ",")
			if len(redisURLs) > 1 || env.GetBool("REDIS_CLUSTER", false) {
				err := r.initClusterRedisClient(redisURLs)
				if err != nil {
					log.Logger().Fatal(err)
				}
			} else {
				err := r.initRedisClient(redisURLs[0])
				if err != nil {
					log.Logger().Fatal(err)
				}
			}
		})
	}
	if prefix == "" {
		r.Prefix = "pusher"
	} else {
		r.Prefix = strings.Trim(prefix, " ")
	}
	return nil
}
