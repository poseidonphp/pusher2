package clients

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"pusher/log"
)

type RedisClient struct {
	Client redis.UniversalClient
	Prefix string
}

var redisOnce sync.Once

func (r *RedisClient) GetKey(key string) string {
	return r.Prefix + ":" + key
}

func (r *RedisClient) GetClient() redis.UniversalClient {
	if r.Client == nil {
		log.Logger().Errorln("Redis Client is not initialized")
	}
	return r.Client
}

func (r *RedisClient) initRedisClient(redisURL string, useTls bool) error {
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
	if useTls {
		universalOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
	r.Client = redis.NewUniversalClient(universalOptions)

	_, err = r.Client.Ping(context.Background()).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) initClusterRedisClient(redisURLs []string, useTls bool) error {
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
	if useTls {
		universalOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
	r.Client = redis.NewUniversalClient(universalOptions)

	_, err = r.Client.Ping(context.Background()).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) InitRedis(redisUrl string, redisCluster bool, useTls bool) error {
	if r.Client == nil {
		// redisOnce.Do(func() {
		redisURLs := strings.Split(redisUrl, ",")
		if len(redisURLs) > 1 || redisCluster {
			err := r.initClusterRedisClient(redisURLs, useTls)
			if err != nil {
				return err
				// log.Logger().Fatal(err)
			}
		} else {
			err := r.initRedisClient(redisURLs[0], useTls)
			if err != nil {
				return err
				// log.Logger().Fatal(err)
			}
		}
		// })
	}

	return nil
}
