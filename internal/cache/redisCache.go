package cache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisCache struct {
	Client redis.UniversalClient
	Prefix string
}

func (r *RedisCache) Init() error {
	if r.Client == nil {
		return errors.New("redis Client is not initialized")
	}
	return nil
}

func (r *RedisCache) getKey(key string) string {
	return r.Prefix + ":cache:" + key
}

func (r *RedisCache) Get(key string) (string, bool) {
	val, err := r.Client.Get(context.Background(), r.getKey(key)).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (r *RedisCache) Set(key string, value string) {
	r.Client.Set(context.Background(), r.getKey(key), value, 0)
}

func (r *RedisCache) SetEx(key string, value string, ttl time.Duration) {
	r.Client.Set(context.Background(), r.getKey(key), value, ttl)
}

func (r *RedisCache) Delete(key string) {
	r.Client.Del(context.Background(), r.getKey(key))
}

func (r *RedisCache) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	val, err := r.Client.Get(context.Background(), r.getKey(key)).Result()
	if err == nil {
		return val, nil
	}
	val, err = callback()
	if err != nil {
		return "", err
	}
	r.Client.Set(context.Background(), r.getKey(key), val, time.Duration(ttl)*time.Second)
	return val, nil
}

func (r *RedisCache) Has(key string) bool {
	exists := r.Client.Exists(context.Background(), r.getKey(key)).Val()
	return exists > 0
}

func (r *RedisCache) Update(key string, value string) {
	_, err := r.Client.Get(context.Background(), r.getKey(key)).Result()
	if err != nil {
		return
	}
	// Update the value only if the key exists
	r.Client.Set(context.Background(), r.getKey(key), value, 0)
}
