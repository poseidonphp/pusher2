package internal

import (
	"fmt"
	"sync"
	"testing"

	"pusher/internal/apps"
	"pusher/internal/constants"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a new namespace for testing
func createTestNamespace() *Namespace {
	return &Namespace{
		Channels: make(map[constants.ChannelName][]constants.SocketID),
		Sockets:  make(map[constants.SocketID]*WebSocket),
		Users:    make(map[string][]constants.SocketID),
	}
}

// Helper function to create a WebSocket for testing
func createTestWebSocket(id constants.SocketID, userID string) *WebSocket {
	return &WebSocket{
		ID:                 id,
		userID:             userID,
		PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
		SubscribedChannels: make(map[constants.ChannelName]*Channel),
		closed:             false,
		conn:               nil, // Set to nil for testing
	}
}

// Helper function to create a test app
func createTestApp() *apps.App {
	app := &apps.App{
		ID:      "test-app",
		Key:     "test-key",
		Secret:  "test-secret",
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// ============================================================================
// SOCKET MANAGEMENT TESTS
// ============================================================================

func TestAddSocket(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socket         *WebSocket
		expectSuccess  bool
		expectCount    int
	}{
		{
			name: "add socket to empty namespace",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:        createTestWebSocket("socket1", ""),
			expectSuccess: true,
			expectCount:   1,
		},
		{
			name: "add socket to namespace with existing sockets",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ns.Sockets["socket1"] = ws1
				return ns
			},
			socket:        createTestWebSocket("socket2", ""),
			expectSuccess: true,
			expectCount:   2,
		},
		{
			name: "add duplicate socket",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ns.Sockets["socket1"] = ws1
				return ns
			},
			socket:        createTestWebSocket("socket1", ""),
			expectSuccess: false,
			expectCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()
			initialCount := len(ns.Sockets)

			success := ns.AddSocket(tt.socket)

			assert.Equal(t, tt.expectSuccess, success)
			assert.Equal(t, tt.expectCount, len(ns.Sockets))
			if tt.expectSuccess {
				assert.Equal(t, tt.socket, ns.Sockets[tt.socket.ID])
			} else {
				assert.Equal(t, initialCount, len(ns.Sockets))
			}
		})
	}
}

func TestRemoveSocket(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socketID       constants.SocketID
		expectRemoved  bool
		expectCount    int
	}{
		{
			name: "remove existing socket",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "user1")
				ws2 := createTestWebSocket("socket2", "")
				ns.Sockets["socket1"] = ws1
				ns.Sockets["socket2"] = ws2
				ns.Users["user1"] = []constants.SocketID{"socket1"}
				ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
				return ns
			},
			socketID:      "socket1",
			expectRemoved: true,
			expectCount:   1,
		},
		{
			name: "remove non-existent socket",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ns.Sockets["socket1"] = ws1
				return ns
			},
			socketID:      "socket2",
			expectRemoved: false,
			expectCount:   1,
		},
		{
			name: "remove socket with user association",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "user1")
				ns.Sockets["socket1"] = ws1
				ns.Users["user1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socketID:      "socket1",
			expectRemoved: true,
			expectCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()
			initialCount := len(ns.Sockets)

			ns.RemoveSocket(tt.socketID)

			if tt.expectRemoved {
				assert.Equal(t, tt.expectCount, len(ns.Sockets))
				assert.NotContains(t, ns.Sockets, tt.socketID)
			} else {
				assert.Equal(t, initialCount, len(ns.Sockets))
			}
		})
	}
}

func TestGetSockets(t *testing.T) {
	ns := createTestNamespace()
	ws1 := createTestWebSocket("socket1", "")
	ws2 := createTestWebSocket("socket2", "")
	ns.Sockets["socket1"] = ws1
	ns.Sockets["socket2"] = ws2

	sockets := ns.GetSockets()

	assert.Equal(t, 2, len(sockets))
	assert.Equal(t, ws1, sockets["socket1"])
	assert.Equal(t, ws2, sockets["socket2"])
}

// ============================================================================
// CHANNEL MANAGEMENT TESTS
// ============================================================================

func TestAddToChannel(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socket         *WebSocket
		channel        constants.ChannelName
		expectCount    int64
	}{
		{
			name: "add socket to new channel",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:      createTestWebSocket("socket1", ""),
			channel:     "channel1",
			expectCount: 1,
		},
		{
			name: "add socket to existing channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socket:      createTestWebSocket("socket2", ""),
			channel:     "channel1",
			expectCount: 2,
		},
		{
			name: "add same socket to channel twice",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socket:      createTestWebSocket("socket1", ""),
			channel:     "channel1",
			expectCount: 2, // Should allow duplicates
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			count := ns.AddToChannel(tt.socket, tt.channel)

			assert.Equal(t, tt.expectCount, count)
			assert.Contains(t, ns.Channels[tt.channel], tt.socket.ID)
		})
	}
}

func TestRemoveFromChannel(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socketID       constants.SocketID
		channels       []constants.ChannelName
		expectCount    int64
	}{
		{
			name: "remove socket from single channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
				return ns
			},
			socketID:    "socket1",
			channels:    []constants.ChannelName{"channel1"},
			expectCount: 1,
		},
		{
			name: "remove socket from multiple channels",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
				ns.Channels["channel2"] = []constants.SocketID{"socket1", "socket3"}
				return ns
			},
			socketID:    "socket1",
			channels:    []constants.ChannelName{"channel1", "channel2"},
			expectCount: 0, // Multiple channels return 0
		},
		{
			name: "remove socket from non-existent channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socketID:    "socket1",
			channels:    []constants.ChannelName{"channel2"},
			expectCount: 0,
		},
		{
			name: "remove last socket from channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Channels["channel1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socketID:    "socket1",
			channels:    []constants.ChannelName{"channel1"},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			count := ns.RemoveFromChannel(tt.socketID, tt.channels)

			assert.Equal(t, tt.expectCount, count)
			for _, channel := range tt.channels {
				if _, exists := ns.Channels[channel]; exists {
					assert.NotContains(t, ns.Channels[channel], tt.socketID)
				}
			}
		})
	}
}

func TestGetChannels(t *testing.T) {
	ns := createTestNamespace()
	ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
	ns.Channels["channel2"] = []constants.SocketID{"socket1"}

	channels := ns.GetChannels()

	assert.Equal(t, 2, len(channels))
	assert.Equal(t, []constants.SocketID{"socket1", "socket2"}, channels["channel1"])
	assert.Equal(t, []constants.SocketID{"socket1"}, channels["channel2"])
}

func TestGetChannelsWithSocketsCount(t *testing.T) {
	ns := createTestNamespace()
	ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2", "socket3"}
	ns.Channels["channel2"] = []constants.SocketID{"socket1"}
	ns.Channels["channel3"] = []constants.SocketID{}

	counts := ns.GetChannelsWithSocketsCount()

	assert.Equal(t, int64(3), counts["channel1"])
	assert.Equal(t, int64(1), counts["channel2"])
	assert.Equal(t, int64(0), counts["channel3"])
}

func TestGetChannelSockets(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		channelName    constants.ChannelName
		expectCount    int
	}{
		{
			name: "get sockets from existing channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ws2 := createTestWebSocket("socket2", "")
				ns.Sockets["socket1"] = ws1
				ns.Sockets["socket2"] = ws2
				ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
				return ns
			},
			channelName: "channel1",
			expectCount: 2,
		},
		{
			name: "get sockets from non-existent channel",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ns.Sockets["socket1"] = ws1
				return ns
			},
			channelName: "channel2",
			expectCount: 0,
		},
		{
			name: "get sockets with disconnected socket",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "")
				ns.Sockets["socket1"] = ws1
				ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"} // socket2 doesn't exist
				return ns
			},
			channelName: "channel1",
			expectCount: 1, // Only socket1 exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			sockets := ns.GetChannelSockets(tt.channelName)

			assert.Equal(t, tt.expectCount, len(sockets))
		})
	}
}

func TestIsInChannel(t *testing.T) {
	ns := createTestNamespace()
	ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}

	tests := []struct {
		name        string
		socketID    constants.SocketID
		channelName constants.ChannelName
		expectIn    bool
	}{
		{
			name:        "socket in channel",
			socketID:    "socket1",
			channelName: "channel1",
			expectIn:    true,
		},
		{
			name:        "socket not in channel",
			socketID:    "socket3",
			channelName: "channel1",
			expectIn:    false,
		},
		{
			name:        "socket in non-existent channel",
			socketID:    "socket1",
			channelName: "channel2",
			expectIn:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ns.IsInChannel(tt.socketID, tt.channelName)
			assert.Equal(t, tt.expectIn, result)
		})
	}
}

// ============================================================================
// USER MANAGEMENT TESTS
// ============================================================================

func TestAddUser(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socket         *WebSocket
		expectError    bool
		expectCount    int
	}{
		{
			name: "add user with valid userID",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:      createTestWebSocket("socket1", "user1"),
			expectError: false,
			expectCount: 1,
		},
		{
			name: "add user with empty userID",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:      createTestWebSocket("socket1", ""),
			expectError: false,
			expectCount: 0, // Empty userID should not be added
		},
		{
			name: "add existing user",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Users["user1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socket:      createTestWebSocket("socket2", "user1"),
			expectError: false,
			expectCount: 2,
		},
		{
			name: "add same socket to same user",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Users["user1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socket:      createTestWebSocket("socket1", "user1"),
			expectError: false,
			expectCount: 1, // Should not duplicate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			err := ns.AddUser(tt.socket)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.socket.userID != "" {
					assert.Equal(t, tt.expectCount, len(ns.Users[tt.socket.userID]))
				}
			}
		})
	}
}

func TestRemoveUser(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		socket         *WebSocket
		expectError    bool
		expectCount    int
		expectDeleted  bool
	}{
		{
			name: "remove user with valid userID",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Users["user1"] = []constants.SocketID{"socket1", "socket2"}
				return ns
			},
			socket:        createTestWebSocket("socket1", "user1"),
			expectError:   false,
			expectCount:   1,
			expectDeleted: false,
		},
		{
			name: "remove last socket from user",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ns.Users["user1"] = []constants.SocketID{"socket1"}
				return ns
			},
			socket:        createTestWebSocket("socket1", "user1"),
			expectError:   false,
			expectCount:   0,
			expectDeleted: true,
		},
		{
			name: "remove user with empty userID",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:        createTestWebSocket("socket1", ""),
			expectError:   false,
			expectCount:   0,
			expectDeleted: false,
		},
		{
			name: "remove non-existent user",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			socket:        createTestWebSocket("socket1", "user1"),
			expectError:   false,
			expectCount:   0,
			expectDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			err := ns.RemoveUser(tt.socket)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.socket.userID != "" {
					if tt.expectDeleted {
						assert.NotContains(t, ns.Users, tt.socket.userID)
					} else {
						assert.Equal(t, tt.expectCount, len(ns.Users[tt.socket.userID]))
					}
				}
			}
		})
	}
}

func TestGetUserSockets(t *testing.T) {
	tests := []struct {
		name           string
		setupNamespace func() *Namespace
		userID         string
		expectCount    int
	}{
		{
			name: "get sockets for existing user",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "user1")
				ws2 := createTestWebSocket("socket2", "user1")
				ns.Sockets["socket1"] = ws1
				ns.Sockets["socket2"] = ws2
				ns.Users["user1"] = []constants.SocketID{"socket1", "socket2"}
				return ns
			},
			userID:      "user1",
			expectCount: 2,
		},
		{
			name: "get sockets for non-existent user",
			setupNamespace: func() *Namespace {
				return createTestNamespace()
			},
			userID:      "user1",
			expectCount: 0,
		},
		{
			name: "get sockets with disconnected socket",
			setupNamespace: func() *Namespace {
				ns := createTestNamespace()
				ws1 := createTestWebSocket("socket1", "user1")
				ns.Sockets["socket1"] = ws1
				ns.Users["user1"] = []constants.SocketID{"socket1", "socket2"} // socket2 doesn't exist
				return ns
			},
			userID:      "user1",
			expectCount: 1, // Only socket1 exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.setupNamespace()

			sockets := ns.GetUserSockets(tt.userID)

			assert.Equal(t, tt.expectCount, len(sockets))
		})
	}
}

// ============================================================================
// PRESENCE CHANNEL TESTS
// ============================================================================

func TestGetPresenceChannelsWithUsersCount(t *testing.T) {
	ns := createTestNamespace()

	// Create WebSockets with presence data
	ws1 := createTestWebSocket("socket1", "user1")
	ws2 := createTestWebSocket("socket2", "user2")
	ws3 := createTestWebSocket("socket3", "user1") // Same user, different socket

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
	ws3.PresenceData = map[constants.ChannelName]*pusherClient.MemberData{
		"presence-channel1": {
			UserID:   "user1",
			UserInfo: map[string]string{"name": "User 1"},
		},
	}

	ns.Sockets["socket1"] = ws1
	ns.Sockets["socket2"] = ws2
	ns.Sockets["socket3"] = ws3
	ns.Channels["presence-channel1"] = []constants.SocketID{"socket1", "socket2", "socket3"}
	ns.Channels["public-channel1"] = []constants.SocketID{"socket1", "socket2"} // Non-presence channel

	counts := ns.GetPresenceChannelsWithUsersCount()

	assert.Equal(t, int64(2), counts["presence-channel1"]) // 2 unique users
	assert.NotContains(t, counts, "public-channel1")       // Non-presence channels should not be included
}

func TestGetChannelMembers(t *testing.T) {
	ns := createTestNamespace()

	// Create WebSockets with presence data
	ws1 := createTestWebSocket("socket1", "user1")
	ws2 := createTestWebSocket("socket2", "user2")

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

	ns.Sockets["socket1"] = ws1
	ns.Sockets["socket2"] = ws2
	ns.Channels["presence-channel1"] = []constants.SocketID{"socket1", "socket2"}

	members := ns.GetChannelMembers("presence-channel1")

	assert.Equal(t, 2, len(members))
	assert.Contains(t, members, "user1")
	assert.Contains(t, members, "user2")
	assert.Equal(t, "user1", members["user1"].UserID)
	assert.Equal(t, "user2", members["user2"].UserID)
}

// ============================================================================
// COMPACT MAPS TESTS
// ============================================================================

func TestCompactMaps(t *testing.T) {
	ns := createTestNamespace()

	// Add some data
	ws1 := createTestWebSocket("socket1", "")
	ws2 := createTestWebSocket("socket2", "")
	ns.Sockets["socket1"] = ws1
	ns.Sockets["socket2"] = ws2
	ns.Sockets["socket3"] = nil // This should be removed
	ns.Channels["channel1"] = []constants.SocketID{"socket1", "socket2"}
	ns.Channels["channel2"] = []constants.SocketID{} // Empty channel should be removed
	ns.Channels["channel3"] = []constants.SocketID{"socket1"}

	ns.CompactMaps()

	// Check that empty channels are removed
	assert.NotContains(t, ns.Channels, "channel2")
	assert.Contains(t, ns.Channels, "channel1")
	assert.Contains(t, ns.Channels, "channel3")

	// Check that nil sockets are removed
	assert.Contains(t, ns.Sockets, "socket1")
	assert.Contains(t, ns.Sockets, "socket2")
	assert.NotContains(t, ns.Sockets, "socket3")
}

// ============================================================================
// CONCURRENCY TESTS
// ============================================================================

func TestConcurrentAddSocket(t *testing.T) {
	ns := createTestNamespace()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			socket := createTestWebSocket(constants.SocketID(fmt.Sprintf("socket%d", i)), "")
			ns.AddSocket(socket)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, numGoroutines, len(ns.Sockets))
}

func TestConcurrentAddToChannel(t *testing.T) {
	ns := createTestNamespace()
	socket := createTestWebSocket("socket1", "")
	ns.AddSocket(socket)

	var wg sync.WaitGroup
	numGoroutines := 100
	channelName := "test-channel"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ns.AddToChannel(socket, channelName)
		}()
	}

	wg.Wait()

	// Should have 100 entries (allows duplicates)
	assert.Equal(t, int64(100), int64(len(ns.Channels[channelName])))
}

func TestConcurrentAddUser(t *testing.T) {
	ns := createTestNamespace()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			socket := createTestWebSocket(constants.SocketID(fmt.Sprintf("socket%d", i)), "user1")
			ns.AddSocket(socket)
			ns.AddUser(socket)
		}(i)
	}

	wg.Wait()

	// All sockets should be added
	assert.Equal(t, numGoroutines, len(ns.Sockets))
	// All sockets should be associated with user1
	assert.Equal(t, numGoroutines, len(ns.Users["user1"]))
}

// ============================================================================
// EDGE CASES AND ERROR CONDITIONS
// ============================================================================

func TestRemoveSocketWithUserCleanup(t *testing.T) {
	ns := createTestNamespace()

	// Create a socket with user association
	ws := createTestWebSocket("socket1", "user1")
	ns.Sockets["socket1"] = ws
	ns.Users["user1"] = []constants.SocketID{"socket1"}
	ns.Channels["channel1"] = []constants.SocketID{"socket1"}

	// Remove the socket
	ns.RemoveSocket("socket1")

	// Check that socket is removed
	assert.NotContains(t, ns.Sockets, "socket1")
	// Check that user is removed
	assert.NotContains(t, ns.Users, "user1")
	// Check that socket is removed from channel
	assert.NotContains(t, ns.Channels["channel1"], "socket1")
}

func TestEmptyNamespaceOperations(t *testing.T) {
	ns := createTestNamespace()

	// Test operations on empty namespace
	assert.Equal(t, 0, len(ns.GetSockets()))
	assert.Equal(t, 0, len(ns.GetChannels()))
	assert.Equal(t, 0, len(ns.GetChannelsWithSocketsCount()))
	assert.Equal(t, 0, len(ns.GetPresenceChannelsWithUsersCount()))

	// Test removing from empty namespace
	count := ns.RemoveFromChannel("socket1", []constants.ChannelName{"channel1"})
	assert.Equal(t, int64(0), count)

	// Test getting sockets for non-existent user
	sockets := ns.GetUserSockets("user1")
	assert.Nil(t, sockets)
}

// ============================================================================
// INTEGRATION TESTS WITH LOCAL ADAPTER
// ============================================================================

func TestNamespaceWithLocalAdapter(t *testing.T) {
	// Create a local adapter
	adapter := &LocalAdapter{}
	err := adapter.Init()
	assert.NoError(t, err)

	// Create a test app
	app := createTestApp()
	appID := constants.AppID(app.ID)

	// Add some test data
	ws1 := createTestWebSocket("socket1", "user1")
	ws2 := createTestWebSocket("socket2", "user2")
	ws1.app = app
	ws2.app = app

	// Test adding sockets through adapter
	err = adapter.AddSocket(appID, ws1)
	assert.NoError(t, err)

	err = adapter.AddSocket(appID, ws2)
	assert.NoError(t, err)

	// Test getting sockets
	sockets := adapter.GetSockets(appID, true)
	assert.Equal(t, 2, len(sockets))

	// Test adding to channels
	count1, err := adapter.AddToChannel(appID, "channel1", ws1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count1)

	count2, err := adapter.AddToChannel(appID, "channel1", ws2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count2)

	// Test getting channel sockets
	channelSockets := adapter.GetChannelSockets(appID, "channel1", true)
	assert.Equal(t, 2, len(channelSockets))

	// Test user management
	err = adapter.AddUser(appID, ws1)
	assert.NoError(t, err)

	err = adapter.AddUser(appID, ws2)
	assert.NoError(t, err)

	// Test getting user sockets
	userSockets, err := adapter.GetUserSockets(appID, "user1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(userSockets))

	// Test removing socket
	err = adapter.RemoveSocket(appID, "socket1")
	assert.NoError(t, err)

	// Verify socket is removed
	sockets = adapter.GetSockets(appID, true)
	assert.Equal(t, 1, len(sockets))
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkAddSocket(b *testing.B) {
	ns := createTestNamespace()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		socket := createTestWebSocket(constants.SocketID(fmt.Sprintf("socket%d", i)), "")
		ns.AddSocket(socket)
	}
}

func BenchmarkAddToChannel(b *testing.B) {
	ns := createTestNamespace()
	socket := createTestWebSocket("socket1", "")
	ns.AddSocket(socket)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ns.AddToChannel(socket, constants.ChannelName(fmt.Sprintf("channel%d", i)))
	}
}

func BenchmarkGetChannelsWithSocketsCount(b *testing.B) {
	ns := createTestNamespace()

	// Set up some data
	for i := 0; i < 100; i++ {
		socket := createTestWebSocket(constants.SocketID(fmt.Sprintf("socket%d", i)), "")
		ns.AddSocket(socket)
		ns.AddToChannel(socket, "channel1")
		if i%2 == 0 {
			ns.AddToChannel(socket, "channel2")
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ns.GetChannelsWithSocketsCount()
	}
}
