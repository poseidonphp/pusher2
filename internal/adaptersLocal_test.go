package internal

import (
	"fmt"
	"sync"
	"testing"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/metrics"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a test app
func createTestAppForAdapter() *apps.App {
	app := &apps.App{
		ID:      "test-app",
		Key:     "test-key",
		Secret:  "test-secret",
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a WebSocket for testing
func createTestWebSocketForAdapter(id constants.SocketID, userID string) *WebSocket {
	return &WebSocket{
		ID:                 id,
		userID:             userID,
		PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
		SubscribedChannels: make(map[constants.ChannelName]*Channel),
		closed:             false,
		conn:               nil,
	}
}

// ============================================================================
// LIFECYCLE TESTS
// ============================================================================

func TestLocalAdapterInit(t *testing.T) {
	adapter := &LocalAdapter{}

	err := adapter.Init()

	assert.NoError(t, err)
	assert.NotNil(t, adapter.Namespaces)
	assert.Equal(t, 0, len(adapter.Namespaces))
}

func TestLocalAdapterDisconnect(t *testing.T) {
	adapter := &LocalAdapter{}
	adapter.Init()

	// Disconnect should not panic and should complete successfully
	adapter.Disconnect()

	// Namespaces should still exist (local adapter doesn't clear on disconnect)
	assert.NotNil(t, adapter.Namespaces)
}

// ============================================================================
// NAMESPACE MANAGEMENT TESTS
// ============================================================================

func TestGetNamespace(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	err := adapter.Init()

	assert.NoError(t, err)

	appID := constants.AppID("test-app")

	// Test getting non-existent namespace
	ns, err := adapter.GetNamespace(appID)
	assert.Error(t, err)
	assert.Nil(t, ns)
	assert.Equal(t, "namespace not found", err.Error())

	// Create a namespace by adding a socket
	ws := createTestWebSocketForAdapter("socket1", "")

	err = adapter.AddSocket(appID, ws)
	assert.NoError(t, err)

	// Now should be able to get the namespace
	ns, err = adapter.GetNamespace(appID)
	assert.NoError(t, err)
	assert.NotNil(t, ns)
}

func TestGetNamespaces(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	// Initially should be empty
	namespaces, err := adapter.GetNamespaces()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(namespaces))

	// Add some namespaces
	appID1 := constants.AppID("app1")
	appID2 := constants.AppID("app2")

	ws1 := createTestWebSocketForAdapter("socket1", "")
	ws2 := createTestWebSocketForAdapter("socket2", "")

	adapter.AddSocket(appID1, ws1)
	adapter.AddSocket(appID2, ws2)

	namespaces, err = adapter.GetNamespaces()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(namespaces))
	assert.Contains(t, namespaces, appID1)
	assert.Contains(t, namespaces, appID2)
}

func TestClearNamespace(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "")

	// Add socket to create namespace
	adapter.AddSocket(appID, ws)

	// Verify namespace exists
	ns, err := adapter.GetNamespace(appID)
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	// Clear namespace
	adapter.ClearNamespace(appID)

	// Namespace should still exist but be empty
	ns, err = adapter.GetNamespace(appID)
	assert.NoError(t, err)
	assert.NotNil(t, ns)
	assert.Equal(t, 0, len(ns.Sockets))
}

func TestClearNamespaces(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	// Add some namespaces
	appID1 := constants.AppID("app1")
	appID2 := constants.AppID("app2")

	ws1 := createTestWebSocketForAdapter("socket1", "")
	ws2 := createTestWebSocketForAdapter("socket2", "")

	adapter.AddSocket(appID1, ws1)
	adapter.AddSocket(appID2, ws2)

	// Verify namespaces exist
	namespaces, _ := adapter.GetNamespaces()
	assert.Equal(t, 2, len(namespaces))

	// Clear all namespaces
	adapter.ClearNamespaces()

	// Should be empty now
	namespaces, _ = adapter.GetNamespaces()
	assert.Equal(t, 0, len(namespaces))
}

// ============================================================================
// SOCKET MANAGEMENT TESTS
// ============================================================================

func TestLocalAdapterAddSocket(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Test adding socket to new namespace
	err := adapter.AddSocket(appID, ws)
	assert.NoError(t, err)

	// Verify socket was added
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, 1, len(sockets))
	assert.Equal(t, ws, sockets["socket1"])

	// Test adding duplicate socket
	err = adapter.AddSocket(appID, ws)
	assert.Error(t, err)
	assert.Equal(t, "socket already exists", err.Error())
}

func TestLocalAdapterRemoveSocket(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket first
	adapter.AddSocket(appID, ws)

	// Verify socket exists
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, 1, len(sockets))

	// Remove socket
	err := adapter.RemoveSocket(appID, "socket1")
	assert.NoError(t, err)

	// Verify socket was removed
	sockets = adapter.GetSockets(appID, true)
	assert.Equal(t, 0, len(sockets))

	// Test removing non-existent socket (should not error)
	err = adapter.RemoveSocket(appID, "socket2")
	assert.NoError(t, err)
}

func TestLocalAdapterGetSockets(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Test getting sockets from non-existent namespace
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, 0, len(sockets))

	// Add sockets
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)

	// Get sockets
	sockets = adapter.GetSockets(appID, true)
	assert.Equal(t, 2, len(sockets))
	assert.Equal(t, ws1, sockets["socket1"])
	assert.Equal(t, ws2, sockets["socket2"])
}

func TestGetSocketsCount(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")

	// Test count from non-existent namespace
	count := adapter.GetSocketsCount(appID, true)
	assert.Equal(t, int64(0), count)

	// Add sockets
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)

	// Get count
	count = adapter.GetSocketsCount(appID, true)
	assert.Equal(t, int64(2), count)
}

// ============================================================================
// CHANNEL MANAGEMENT TESTS
// ============================================================================

func TestLocalAdapterAddToChannel(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket first
	adapter.AddSocket(appID, ws)

	// Test adding to channel
	count, err := adapter.AddToChannel(appID, "channel1", ws)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test adding to non-existent namespace
	nonExistentAppID := constants.AppID("non-existent")
	count, err = adapter.AddToChannel(nonExistentAppID, "channel1", ws)
	assert.Error(t, err)
	assert.Equal(t, int64(0), count)
	assert.Equal(t, "namespace not found", err.Error())
}

func TestLocalAdapterRemoveFromChannel(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket and channel first
	adapter.AddSocket(appID, ws)
	adapter.AddToChannel(appID, "channel1", ws)

	// Test removing from channel
	count := adapter.RemoveFromChannel(appID, []constants.ChannelName{"channel1"}, "socket1")
	assert.Equal(t, int64(0), count)

	// Test removing from non-existent namespace
	count = adapter.RemoveFromChannel(constants.AppID("non-existent"), []constants.ChannelName{"channel1"}, "socket1")
	assert.Equal(t, int64(0), count)
}

func TestLocalAdapterGetChannels(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Test getting channels from non-existent namespace
	channels := adapter.GetChannels(appID, true)
	assert.Nil(t, channels)

	// Add socket and channels
	adapter.AddSocket(appID, ws)
	adapter.AddToChannel(appID, "channel1", ws)
	adapter.AddToChannel(appID, "channel2", ws)

	// Get channels
	channels = adapter.GetChannels(appID, true)
	assert.Equal(t, 2, len(channels))
	assert.Contains(t, channels, "channel1")
	assert.Contains(t, channels, "channel2")
}

func TestLocalAdapterGetChannelsWithSocketsCount(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Test getting counts from non-existent namespace
	counts := adapter.GetChannelsWithSocketsCount(appID, true)
	assert.Equal(t, 0, len(counts))

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "channel1", ws1)
	adapter.AddToChannel(appID, "channel1", ws2)
	adapter.AddToChannel(appID, "channel2", ws1)

	// Get counts
	counts = adapter.GetChannelsWithSocketsCount(appID, true)
	assert.Equal(t, int64(2), counts["channel1"])
	assert.Equal(t, int64(1), counts["channel2"])
}

func TestLocalAdapterGetChannelSockets(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Test getting sockets from non-existent namespace
	sockets := adapter.GetChannelSockets(appID, "channel1", true)
	assert.Equal(t, 0, len(sockets))

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "channel1", ws1)
	adapter.AddToChannel(appID, "channel1", ws2)

	// Get channel sockets
	sockets = adapter.GetChannelSockets(appID, "channel1", true)
	assert.Equal(t, 2, len(sockets))
	assert.Equal(t, ws1, sockets["socket1"])
	assert.Equal(t, ws2, sockets["socket2"])
}

func TestGetChannelSocketsCount(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Test getting count from non-existent namespace
	count := adapter.GetChannelSocketsCount(appID, "channel1", true)
	assert.Equal(t, int64(0), count)

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "channel1", ws1)
	adapter.AddToChannel(appID, "channel1", ws2)

	// Get count
	count = adapter.GetChannelSocketsCount(appID, "channel1", true)
	assert.Equal(t, int64(2), count)
}

func TestLocalAdapterIsInChannel(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Test checking non-existent namespace
	result := adapter.IsInChannel(appID, "channel1", "socket1", true)
	assert.False(t, result)

	// Add socket and channel
	adapter.AddSocket(appID, ws)
	adapter.AddToChannel(appID, "channel1", ws)

	// Test checking membership
	result = adapter.IsInChannel(appID, "channel1", "socket1", true)
	assert.True(t, result)

	result = adapter.IsInChannel(appID, "channel1", "socket2", true)
	assert.False(t, result)
}

// ============================================================================
// USER MANAGEMENT TESTS
// ============================================================================

func TestLocalAdapterAddUser(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket first
	adapter.AddSocket(appID, ws)

	// Test adding user
	err := adapter.AddUser(appID, ws)
	assert.NoError(t, err)

	// Test adding user to non-existent namespace
	err = adapter.AddUser(constants.AppID("non-existent"), ws)
	assert.Error(t, err)
	assert.Equal(t, "namespace not found", err.Error())
}

func TestLocalAdapterRemoveUser(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket and user
	adapter.AddSocket(appID, ws)
	adapter.AddUser(appID, ws)

	// Test removing user
	err := adapter.RemoveUser(appID, ws)
	assert.NoError(t, err)

	// Test removing user from non-existent namespace
	err = adapter.RemoveUser(constants.AppID("non-existent"), ws)
	assert.Error(t, err)
	assert.Equal(t, "namespace not found", err.Error())
}

func TestLocalAdapterGetUserSockets(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user1")

	// Test getting user sockets from non-existent namespace
	sockets, err := adapter.GetUserSockets(appID, "user1")
	assert.Error(t, err)
	assert.Nil(t, sockets)
	assert.Equal(t, "namespace not found", err.Error())

	// Add sockets and users
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddUser(appID, ws1)
	adapter.AddUser(appID, ws2)

	// Get user sockets
	sockets, err = adapter.GetUserSockets(appID, "user1")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(sockets))
}

// ============================================================================
// PRESENCE CHANNEL TESTS
// ============================================================================

func TestLocalAdapterGetChannelMembers(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Set up presence data
	ws1.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user1",
			UserInfo: map[string]string{"name": "User 1"},
		},
	}
	ws2.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user2",
			UserInfo: map[string]string{"name": "User 2"},
		},
	}

	// Test getting members from non-existent namespace
	members := adapter.GetChannelMembers(appID, "presence-channel1", true)
	assert.Equal(t, 0, len(members))

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "presence-channel1", ws1)
	adapter.AddToChannel(appID, "presence-channel1", ws2)

	// Get channel members
	members = adapter.GetChannelMembers(appID, "presence-channel1", true)
	assert.Equal(t, 2, len(members))
	assert.Contains(t, members, "user1")
	assert.Contains(t, members, "user2")
}

func TestGetChannelMembersCount(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Set up presence data
	ws1.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user1",
			UserInfo: map[string]string{"name": "User 1"},
		},
	}
	ws2.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user2",
			UserInfo: map[string]string{"name": "User 2"},
		},
	}

	// Test getting count from non-existent namespace
	count := adapter.GetChannelMembersCount(appID, "presence-channel1", true)
	assert.Equal(t, 0, count)

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "presence-channel1", ws1)
	adapter.AddToChannel(appID, "presence-channel1", ws2)

	// Get count
	count = adapter.GetChannelMembersCount(appID, "presence-channel1", true)
	assert.Equal(t, 2, count)
}

func TestLocalAdapterGetPresenceChannelsWithUsersCount(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Set up presence data
	ws1.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user1",
			UserInfo: map[string]string{"name": "User 1"},
		},
	}
	ws2.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user2",
			UserInfo: map[string]string{"name": "User 2"},
		},
	}

	// Test getting counts from non-existent namespace
	counts := adapter.GetPresenceChannelsWithUsersCount(appID, true)
	assert.Equal(t, 0, len(counts))

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "presence-channel1", ws1)
	adapter.AddToChannel(appID, "presence-channel1", ws2)
	adapter.AddToChannel(appID, "public-channel1", ws1) // Non-presence channel

	// Get counts
	counts = adapter.GetPresenceChannelsWithUsersCount(appID, true)
	assert.Equal(t, int64(2), counts["presence-channel1"])
	assert.NotContains(t, counts, "public-channel1")
}

// ============================================================================
// MESSAGING TESTS
// ============================================================================

func TestSend(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user2")

	// Test sending to non-existent namespace
	err := adapter.Send(appID, "channel1", []byte("test message"))
	assert.Error(t, err)
	assert.Equal(t, "namespace not found", err.Error())

	// Add sockets and channels
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddToChannel(appID, "channel1", ws1)
	adapter.AddToChannel(appID, "channel1", ws2)

	// Test sending to channel
	err = adapter.Send(appID, "channel1", []byte("test message"))
	assert.NoError(t, err)

	// Test sending with exception
	err = adapter.Send(appID, "channel1", []byte("test message"), "socket1")
	assert.NoError(t, err)
}

func TestSendToUserChannel(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Add socket and user
	adapter.AddSocket(appID, ws)
	adapter.AddUser(appID, ws)

	// Test sending to user channel
	userChannel := constants.SocketRushServerToUserPrefix + "user1"
	err := adapter.Send(appID, userChannel, []byte("test message"))
	assert.NoError(t, err)
}

func TestTerminateUserConnections(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws1 := createTestWebSocketForAdapter("socket1", "user1")
	ws2 := createTestWebSocketForAdapter("socket2", "user1")
	ws3 := createTestWebSocketForAdapter("socket3", "user2")

	// Add sockets and users
	adapter.AddSocket(appID, ws1)
	adapter.AddSocket(appID, ws2)
	adapter.AddSocket(appID, ws3)
	adapter.AddUser(appID, ws1)
	adapter.AddUser(appID, ws2)
	adapter.AddUser(appID, ws3)

	// Test terminating user connections
	// TODO: We can't do this until we have mocked or built websocket connections that can be closed
	// adapter.TerminateUserConnections(appID, "user1")

	// Note: We can't easily test the actual termination without mocking WebSocket.closeConnection
	// The method should complete without error
}

// ============================================================================
// CONCURRENCY TESTS
// ============================================================================

func TestConcurrentOperations(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")

	var wg sync.WaitGroup
	numGoroutines := 50

	// Test concurrent socket additions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ws := createTestWebSocketForAdapter(constants.SocketID(fmt.Sprintf("socket%d", i)), "")
			adapter.AddSocket(appID, ws)
		}(i)
	}

	wg.Wait()

	// Verify all sockets were added
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, numGoroutines, len(sockets))
}

// ============================================================================
// EDGE CASES AND ERROR CONDITIONS
// ============================================================================

func TestEmptyAdapterOperations(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")

	// Test operations on empty adapter
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, 0, len(sockets))

	count := adapter.GetSocketsCount(appID, true)
	assert.Equal(t, int64(0), count)

	channels := adapter.GetChannels(appID, true)
	assert.Nil(t, channels)

	channelCounts := adapter.GetChannelsWithSocketsCount(appID, true)
	assert.Equal(t, 0, len(channelCounts))

	presenceCounts := adapter.GetPresenceChannelsWithUsersCount(appID, true)
	assert.Equal(t, 0, len(presenceCounts))
}

func TestNamespaceAutoCreation(t *testing.T) {
	adapter := NewLocalAdapter(&metrics.NoOpMetrics{})
	adapter.Init()

	appID := constants.AppID("test-app")
	ws := createTestWebSocketForAdapter("socket1", "user1")

	// Adding a socket should auto-create the namespace
	err := adapter.AddSocket(appID, ws)
	assert.NoError(t, err)

	// Namespace should now exist
	ns, err := adapter.GetNamespace(appID)
	assert.NoError(t, err)
	assert.NotNil(t, ns)
}
