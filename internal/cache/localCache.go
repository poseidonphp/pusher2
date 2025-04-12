package cache

import (
	"sync"
	"time"
)

type localCacheItem struct {
	data     string
	expireAt *time.Time
}

type LocalCache struct {
	cacheItems map[string]*localCacheItem
	mu         sync.Mutex
}

func (l *LocalCache) Init() error {
	l.cacheItems = make(map[string]*localCacheItem)
	go l.cleanupJob()
	return nil
}

func (l *LocalCache) cleanupJob() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, item := range l.cacheItems {
			if item.expireAt != nil && now.After(*item.expireAt) {
				delete(l.cacheItems, key)
			}
		}
		l.mu.Unlock()
	}
}

func (l *LocalCache) existsAndNotExpired(key string) (exists bool, item *localCacheItem) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if item, exists = l.cacheItems[key]; exists {
		if item.expireAt == nil || time.Now().Before(*item.expireAt) {
			return true, item
		}
	}
	return false, nil
}

func (l *LocalCache) Get(key string) (string, bool) {
	if exists, item := l.existsAndNotExpired(key); exists {
		return item.data, true
	}
	return "", false
}

func (l *LocalCache) Set(key string, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cacheItems[key] = &localCacheItem{
		data: value,
	}
}

func (l *LocalCache) SetEx(key string, value string, ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	expTime := time.Now().Add(ttl)
	l.cacheItems[key] = &localCacheItem{
		data:     value,
		expireAt: &expTime,
	}
}

func (l *LocalCache) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.cacheItems[key]; exists {
		delete(l.cacheItems, key)
	}
}

func (l *LocalCache) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	if exists, item := l.existsAndNotExpired(key); exists {
		return item.data, nil
	}
	val, err := callback()
	if err != nil {
		return "", err
	}
	l.SetEx(key, val, time.Duration(ttl)*time.Second)
	return val, nil
}

func (l *LocalCache) Has(key string) bool {
	if exists, _ := l.existsAndNotExpired(key); exists {
		return true
	}

	return false
}

func (l *LocalCache) Update(key string, value string) {
	if exists, _ := l.existsAndNotExpired(key); exists {
		l.Set(key, value)
	}
}
