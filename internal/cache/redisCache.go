package cache

import (
	"github.com/go-redis/redis"
	"pusher/internal/clients"
	"time"
)

type RedisCache struct {
	client redis.UniversalClient
	prefix string
}

func (r *RedisCache) Init() error {
	r.client = clients.RedisClientInstance.GetClient()
	r.prefix = clients.RedisClientInstance.Prefix

	return nil
}

func (r *RedisCache) getKey(key string) string {
	return r.prefix + ":cache:" + key
}

func (r *RedisCache) Get(key string) (string, bool) {
	val, err := r.client.Get(r.getKey(key)).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (r *RedisCache) Set(key string, value string) {
	r.client.Set(r.getKey(key), value, 0)
}

func (r *RedisCache) SetEx(key string, value string, ttl time.Duration) {
	r.client.Set(r.getKey(key), value, ttl)
}

func (r *RedisCache) Delete(key string) {
	r.client.Del(r.getKey(key))
}

func (r *RedisCache) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	val, err := r.client.Get(r.getKey(key)).Result()
	if err == nil {
		return val, nil
	}
	val, err = callback()
	if err != nil {
		return "", err
	}
	r.client.Set(r.getKey(key), val, time.Duration(ttl)*time.Second)
	return val, nil
}

func (r *RedisCache) Has(key string) bool {
	exists := r.client.Exists(r.getKey(key)).Val()
	return exists > 0
}

func (r *RedisCache) Update(key string, value string) {
	if r.Has(r.getKey(key)) {
		r.Set(r.getKey(key), value)
	}
}
