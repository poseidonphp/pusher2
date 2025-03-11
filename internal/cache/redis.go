package cache

import (
	"github.com/go-redis/redis"
	"time"
)

type RedisCache struct {
	client redis.UniversalClient
}

func (r *RedisCache) Get(key string) (string, bool) {
	val, err := r.client.Get(key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (r *RedisCache) Set(key string, value string) {
	r.client.Set(key, value, 0)
}

func (r *RedisCache) Delete(key string) {
	r.client.Del(key)
}

func (r *RedisCache) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	val, err := r.client.Get(key).Result()
	if err == nil {
		return val, nil
	}
	val, err = callback()
	if err != nil {
		return "", err
	}
	r.client.Set(key, val, time.Duration(ttl)*time.Second)
	return val, nil
}

func (r *RedisCache) Has(key string) bool {
	exists := r.client.Exists(key).Val()
	return exists > 0
}

func (r *RedisCache) Update(key string, value string) {
	if r.Has(key) {
		r.Set(key, value)
	}
}
