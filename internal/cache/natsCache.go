package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// NatsCache implements CacheContract on top of a NATS JetStream KV bucket.
//
// Expiration (SetEx) is tracked ourselves, the same way LocalCache does it
// (an expiry timestamp checked on Get, deleting the entry if it has
// passed), rather than relying on the server's own per-key TTL support.
// That keeps it working the same way regardless of the JetStream server
// version in use.
type NatsCache struct {
	Client jetstream.JetStream
	Prefix string
	bucket jetstream.KeyValue
}

type natsCacheEntry struct {
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expiresAt,omitempty"` // unix nanoseconds; 0 = no expiration
}

func (n *NatsCache) Init(ctx context.Context) error {
	if n.Client == nil {
		return errors.New("nats JetStream client is not initialized")
	}
	bucketName := n.Prefix
	if bucketName == "" {
		bucketName = "pusher"
	}
	bucketName += "_cache"

	bucket, err := n.Client.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucketName})
	if err != nil {
		return err
	}
	n.bucket = bucket
	return nil
}

func (n *NatsCache) Shutdown() {
	// NATS cache has no background goroutines, so no cleanup needed.
}

// getKey builds the bucket key for a cache key. NATS KV keys only allow
// alphanumerics, '-', '_', '=' and '.', so any '#' or ':' from the shared
// GetChannelCacheKey format (or caller-supplied keys) is sanitized.
func (n *NatsCache) getKey(key string) string {
	sanitized := strings.NewReplacer("#", "_", ":", "_").Replace(key)
	return "cache." + sanitized
}

func (n *NatsCache) Get(key string) (string, bool) {
	entry, exists := n.getEntry(key)
	if !exists {
		return "", false
	}
	return entry.Value, true
}

// getEntry fetches and decodes the entry, deleting it (and reporting it as
// missing) if it has expired.
func (n *NatsCache) getEntry(key string) (natsCacheEntry, bool) {
	kvKey := n.getKey(key)
	res, err := n.bucket.Get(context.Background(), kvKey)
	if err != nil {
		return natsCacheEntry{}, false
	}

	var entry natsCacheEntry
	if err := json.Unmarshal(res.Value(), &entry); err != nil {
		return natsCacheEntry{}, false
	}

	if entry.ExpiresAt != 0 && time.Now().UnixNano() >= entry.ExpiresAt {
		_ = n.bucket.Delete(context.Background(), kvKey)
		return natsCacheEntry{}, false
	}
	return entry, true
}

func (n *NatsCache) putEntry(key string, entry natsCacheEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = n.bucket.Put(context.Background(), n.getKey(key), data)
}

func (n *NatsCache) Set(key string, value string) {
	n.putEntry(key, natsCacheEntry{Value: value})
}

func (n *NatsCache) SetEx(key string, value string, ttl time.Duration) {
	n.putEntry(key, natsCacheEntry{Value: value, ExpiresAt: time.Now().Add(ttl).UnixNano()})
}

func (n *NatsCache) Delete(key string) {
	_ = n.bucket.Delete(context.Background(), n.getKey(key))
}

func (n *NatsCache) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
	if entry, exists := n.getEntry(key); exists {
		return entry.Value, nil
	}
	val, err := callback()
	if err != nil {
		return "", err
	}
	n.SetEx(key, val, time.Duration(ttl)*time.Second)
	return val, nil
}

func (n *NatsCache) Has(key string) bool {
	_, exists := n.getEntry(key)
	return exists
}

func (n *NatsCache) Update(key string, value string) {
	if _, exists := n.getEntry(key); exists {
		n.Set(key, value)
	}
}
