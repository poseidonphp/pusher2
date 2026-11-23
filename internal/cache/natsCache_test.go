package cache

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestNats starts an embedded NATS server with JetStream enabled (the
// cache package's own copy of the helper in internal/clients, since test
// helpers aren't exported across packages) and returns a connected
// JetStream context, cleaned up automatically when the test ends.
func setupTestNats(t *testing.T) jetstream.JetStream {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      server.RANDOM_PORT,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second), "nats server did not become ready in time")
	t.Cleanup(ns.Shutdown)

	conn, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	js, err := jetstream.New(conn)
	require.NoError(t, err)

	return js
}

func TestNatsCache(t *testing.T) {
	natsCache := &NatsCache{
		Client: nil,
		Prefix: "test",
	}
	t.Run("InitEmpty", func(t *testing.T) {
		err := natsCache.Init(t.Context())
		assert.Error(t, err)
		assert.Equal(t, "nats JetStream client is not initialized", err.Error())
	})

	js := setupTestNats(t)
	natsCache = &NatsCache{
		Client: js,
		Prefix: "test",
	}

	t.Run("InitValid", func(t *testing.T) {
		err := natsCache.Init(t.Context())
		assert.NoError(t, err)
	})

	t.Run("GetKey", func(t *testing.T) {
		key := natsCache.getKey("mykey")
		assert.Equal(t, "cache.mykey", key)
	})

	t.Run("GetKeySanitizesReservedChars", func(t *testing.T) {
		key := natsCache.getKey(GetChannelCacheKey("myprefix", "my-channel"))
		assert.Equal(t, "cache.app_myprefix_channel_my-channel", key)
	})

	t.Run("SetExWithExpiration", func(t *testing.T) {
		natsCache.SetEx("keyex", "value-with-ttl", 500*time.Millisecond)

		val, exists := natsCache.Get("keyex")
		assert.True(t, exists)
		assert.Equal(t, "value-with-ttl", val)

		time.Sleep(600 * time.Millisecond)

		val, exists = natsCache.Get("keyex")
		assert.False(t, exists)
		assert.Equal(t, "", val)
	})

	cacheFactory := func() CacheContract {
		return natsCache
	}
	RunCacheTests(t, "NatsCache", cacheFactory)
}

func TestNatsCache_InitCreatesBucketOnce(t *testing.T) {
	js := setupTestNats(t)
	natsCache := &NatsCache{Client: js, Prefix: "test-once"}

	require.NoError(t, natsCache.Init(context.Background()))
	// Re-initializing (e.g. on reconnect) against an already-existing
	// bucket should not error.
	require.NoError(t, natsCache.Init(context.Background()))
}
