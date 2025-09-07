package clients

import (
	"context"
	"crypto/tls"
	"fmt"
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

func (r *RedisClient) ParseUrlAndGetOptions(redisURL string, forceTls bool) (*redis.UniversalOptions, error) {
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url %s: %s", redisURL, err.Error())
	}

	opts := &redis.UniversalOptions{
		Addrs:       []string{redisOptions.Addr},
		DB:          redisOptions.DB,
		Password:    redisOptions.Password,
		PoolSize:    redisOptions.PoolSize,
		PoolTimeout: redisOptions.PoolTimeout,
		TLSConfig:   redisOptions.TLSConfig,
	}
	if forceTls && opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
	return opts, nil
}

func (r *RedisClient) initRedisClient(redisURL string, useTls bool) error {
	universalOptions, err := r.ParseUrlAndGetOptions(redisURL, useTls)
	if err != nil {
		return err
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

	// Parse the first URL to get common options like Password, DB, etc.
	universalOptions, err := r.ParseUrlAndGetOptions(redisURLs[0], useTls)
	if err != nil {
		return fmt.Errorf("failed to parse redis url %s: %s", redisURLs[0], err.Error())
	}

	// Parse all URLs to get the addresses
	for _, redisURL := range redisURLs {
		uniOptions, pErr := r.ParseUrlAndGetOptions(redisURL, useTls)
		if pErr != nil {
			return pErr
		}
		nodes = append(nodes, uniOptions.Addrs[0])
	}
	universalOptions.Addrs = nodes

	r.Client = redis.NewUniversalClient(universalOptions)

	_, err = r.Client.Ping(context.Background()).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) InitRedis(redisUrl string, redisCluster bool, useTls bool) error {
	if r.Client == nil {
		redisURLs := strings.Split(redisUrl, ",")
		if len(redisURLs) > 1 || redisCluster {
			return r.initClusterRedisClient(redisURLs, useTls)
		} else {
			return r.initRedisClient(redisURLs[0], useTls)
		}
	}
	return nil
}
