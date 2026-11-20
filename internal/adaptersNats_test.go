package internal

import (
	"context"
	"testing"
	"time"

	"pusher/internal/constants"
	"pusher/internal/metrics"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestNatsServer starts an embedded NATS server on a random free port,
// the same role miniredis plays for the Redis adapter tests. JetStream
// isn't needed here (only the NATS-backed cache uses it), just core
// pub/sub.
func setupTestNatsServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second), "nats server did not become ready in time")
	t.Cleanup(ns.Shutdown)

	return ns
}

func connectTestNats(t *testing.T, ns *server.Server) *nats.Conn {
	t.Helper()
	conn, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	return conn
}

// createTestNatsAdapter creates a NatsAdapter against a fresh embedded
// NATS server, the NATS equivalent of createTestRedisAdapter.
func createTestNatsAdapter(t *testing.T) *NatsAdapter {
	t.Helper()
	ns := setupTestNatsServer(t)
	conn := connectTestNats(t, ns)

	adapter, err := NewNatsAdapter(context.Background(), conn, "test-prefix", "test-channel", &metrics.NoOpMetrics{})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	return adapter
}

func TestNewNatsAdapter(t *testing.T) {
	ns := setupTestNatsServer(t)
	conn := connectTestNats(t, ns)

	adapter, err := NewNatsAdapter(context.Background(), conn, "test-prefix", "test-channel", &metrics.NoOpMetrics{})
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, "test-prefix.test-channel", adapter.Channel)
	assert.Equal(t, "test-prefix.test-channel.comms.req", adapter.requestChannel)
	assert.Equal(t, "test-prefix.test-channel.comms.res", adapter.responseChannel)
	assert.NotNil(t, adapter.HorizontalAdapter)

	// Test with empty prefix
	adapter2, err := NewNatsAdapter(context.Background(), conn, "", "test-channel", &metrics.NoOpMetrics{})
	assert.NoError(t, err)
	assert.Equal(t, "test-channel", adapter2.Channel)
}

func TestNatsAdapterInitWithNilConn(t *testing.T) {
	adapter := &NatsAdapter{
		ctx:     context.Background(),
		conn:    nil,
		Channel: "test-channel",
		peers:   make(map[string]time.Time),
	}

	err := adapter.Init()
	assert.Error(t, err)
	assert.Equal(t, "nats connection not initialized", err.Error())
}

func TestNatsAdapterGetChannelName(t *testing.T) {
	adapter := createTestNatsAdapter(t)
	assert.Equal(t, "test-prefix.test-channel.msg", adapter.GetChannelName())
}

func TestNatsAdapterBroadcastToChannel(t *testing.T) {
	adapter := createTestNatsAdapter(t)

	// Should not error/panic.
	adapter.broadcastToChannel("some.subject", "test message")
}

func TestNatsAdapterGetNumSub_Alone(t *testing.T) {
	adapter := createTestNatsAdapter(t)

	count, err := adapter.getNumSub()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestNatsAdapterPeerPresence proves the core NATS-specific behavior this
// adapter adds: since NATS has no server-side subscriber-count query (the
// equivalent of Redis's PUBSUB NUMSUB), getNumSub() is backed by a
// heartbeat each node publishes on a presence subject. Two real adapters
// against the same embedded NATS server should discover each other and
// both report a count of 2 within a couple of heartbeat ticks.
func TestNatsAdapterPeerPresence(t *testing.T) {
	ns := setupTestNatsServer(t)

	conn1 := connectTestNats(t, ns)
	adapter1, err := NewNatsAdapter(context.Background(), conn1, "test-prefix", "int", &metrics.NoOpMetrics{})
	require.NoError(t, err)

	conn2 := connectTestNats(t, ns)
	adapter2, err := NewNatsAdapter(context.Background(), conn2, "test-prefix", "int", &metrics.NoOpMetrics{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		c1, _ := adapter1.getNumSub()
		c2, _ := adapter2.getNumSub()
		return c1 == 2 && c2 == 2
	}, 5*time.Second, 50*time.Millisecond, "both adapters should see a peer count of 2")
}

// TestNatsAdapterPeerEviction proves stale peers are dropped once they
// stop heartbeating (e.g. the other node went away).
func TestNatsAdapterPeerEviction(t *testing.T) {
	ns := setupTestNatsServer(t)

	conn1 := connectTestNats(t, ns)
	adapter1, err := NewNatsAdapter(context.Background(), conn1, "test-prefix", "int", &metrics.NoOpMetrics{})
	require.NoError(t, err)

	conn2 := connectTestNats(t, ns)
	adapter2, err := NewNatsAdapter(context.Background(), conn2, "test-prefix", "int", &metrics.NoOpMetrics{})
	require.NoError(t, err)
	_ = adapter2

	require.Eventually(t, func() bool {
		c, _ := adapter1.getNumSub()
		return c == 2
	}, 5*time.Second, 50*time.Millisecond, "adapter1 should see adapter2")

	// Simulate adapter2 disappearing without a graceful disconnect.
	conn2.Close()

	// Eviction needs the peer to go a full natsPresenceTTL without a
	// heartbeat, plus up to one more sweep interval before the next sweep
	// notices; give it comfortable room above that worst case.
	require.Eventually(t, func() bool {
		c, _ := adapter1.getNumSub()
		return c == 1
	}, natsPresenceTTL+4*time.Second, 50*time.Millisecond, "adapter1 should evict adapter2 after it stops heartbeating")
}

func TestNatsAdapterOnMessage_Broadcast(t *testing.T) {
	adapter := createTestNatsAdapter(t)

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForRedis("socket1", "user1")
	require.NoError(t, adapter.AddSocket(appID, ws))
	_, err := adapter.AddToChannel(appID, "channel1", ws)
	require.NoError(t, err)

	validMessage := `{
		"Uuid": "different-uuid",
		"AppID": "test-app",
		"Channel": "channel1",
		"Data": "test data",
		"ExceptingIds": ""
	}`
	adapter.onMessage(&nats.Msg{Subject: adapter.GetChannelName(), Data: []byte(validMessage)})

	// Message from ourselves should be ignored, not delivered.
	sameUUIDMessage := `{
		"Uuid": "` + adapter.uuid + `",
		"AppID": "test-app",
		"Channel": "channel1",
		"Data": "test data",
		"ExceptingIds": ""
	}`
	adapter.onMessage(&nats.Msg{Subject: adapter.GetChannelName(), Data: []byte(sameUUIDMessage)})

	// Invalid JSON and messages on unrelated subjects should not panic.
	adapter.onMessage(&nats.Msg{Subject: adapter.GetChannelName(), Data: []byte("invalid json")})
	adapter.onMessage(&nats.Msg{Subject: "unexpected.subject", Data: []byte(validMessage)})
}

func TestNatsAdapterOnMessage_Presence(t *testing.T) {
	adapter := createTestNatsAdapter(t)

	adapter.onMessage(&nats.Msg{Subject: adapter.presenceSubject(), Data: []byte("peer-uuid-1")})

	adapter.peersMu.Lock()
	_, exists := adapter.peers["peer-uuid-1"]
	adapter.peersMu.Unlock()
	assert.True(t, exists)

	// A self-announcement should never be recorded as a peer.
	adapter.onMessage(&nats.Msg{Subject: adapter.presenceSubject(), Data: []byte(adapter.uuid)})
	adapter.peersMu.Lock()
	_, selfExists := adapter.peers[adapter.uuid]
	adapter.peersMu.Unlock()
	assert.False(t, selfExists)
}

func TestNatsAdapterSend(t *testing.T) {
	adapter := createTestNatsAdapter(t)

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForRedis("socket1", "user1")
	require.NoError(t, adapter.AddSocket(appID, ws))
	_, chErr := adapter.AddToChannel(appID, "channel1", ws)
	require.NoError(t, chErr)

	err := adapter.Send(appID, "channel1", []byte("test message"))
	assert.NoError(t, err)
}
