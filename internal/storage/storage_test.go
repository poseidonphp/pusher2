package storage

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pusher/internal/constants"
	"pusher/internal/pubsub"
)

// setupMiniRedis creates a miniredis instance and client for testing
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

func TestLocalStorage(t *testing.T) {
	// create a local pubsub manager as it is needed when using local storage
	localPubsub := &pubsub.StandAlonePubSubManager{}
	localStorage := &StandaloneStorageManager{Pubsub: localPubsub}
	err := localStorage.Init()
	require.NoError(t, err)

	storageFactory := func() StorageContract {
		return localStorage
	}

	RunStorageTests(t, "LocalStorage", storageFactory)
}

func TestRedisStorage(t *testing.T) {
	// Setup miniredis for testing
	mr, client := setupMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	redisStorage := &RedisStorage{
		Client:    client,
		KeyPrefix: "test",
	}

	err := redisStorage.Init()
	require.NoError(t, err)

	// Test redisStorage-specific methods
	t.Run("GetKeyName", func(t *testing.T) {
		key := redisStorage.getKey("test-key")
		assert.Equal(t, "test:test-key", key)
	})

	storageFactory := func() StorageContract {
		return redisStorage
	}

	RunStorageTests(t, "RedisStorage", storageFactory)
}

func RunStorageTests(t *testing.T, name string, factory func() StorageContract) {
	t.Run(name, func(t *testing.T) {
		storage := factory()

		// Create a context for operations
		_ = context.Background()

		// Define test node IDs
		node1 := constants.NodeID("node1")
		node2 := constants.NodeID("node2")

		// Define test channels
		channel1 := constants.ChannelName("test-channel1")
		channel2 := constants.ChannelName("test-channel2")
		presenceChannel := constants.ChannelName("presence-channel")

		// Define test socket IDs
		socket1 := constants.SocketID("socket1")
		socket2 := constants.SocketID("socket2")
		socket3 := constants.SocketID("socket3")

		// Set up some sample members for presence tests
		memberData1 := pusherClient.MemberData{
			UserID: "user1",
			UserInfo: map[string]string{
				"name": "Test User 1",
			},
		}

		memberData2 := pusherClient.MemberData{
			UserID: "user2",
			UserInfo: map[string]string{
				"name": "Test User 2",
			},
		}

		t.Run("NodeOperations", func(t *testing.T) {
			t.Run("AddAndGetNodes", func(t *testing.T) {
				// Add nodes
				err := storage.AddNewNode(node1)
				assert.NoError(t, err)

				err = storage.AddNewNode(node2)
				assert.NoError(t, err)

				// Get all nodes
				nodes, err := storage.GetAllNodes()
				assert.NoError(t, err)
				assert.Contains(t, nodes, string(node1))
				assert.Contains(t, nodes, string(node2))
				assert.Len(t, nodes, 2)
			})

			t.Run("RemoveNode", func(t *testing.T) {
				// Remove a node
				err := storage.RemoveNode(node1)
				assert.NoError(t, err)

				// Verify it's gone
				nodes, err := storage.GetAllNodes()
				assert.NoError(t, err)
				assert.NotContains(t, nodes, string(node1))
				assert.Contains(t, nodes, string(node2))

				// Re-add for subsequent tests
				err = storage.AddNewNode(node1)
				assert.NoError(t, err)
			})

			t.Run("NodeHeartbeat", func(t *testing.T) {
				// Send heartbeat
				timestamp := storage.SendNodeHeartbeat(node1)
				assert.NotNil(t, timestamp)

				// Current implementation likely just returns current time,
				// so we can't verify much beyond that it returns a timestamp
				assert.WithinDuration(t, time.Now(), *timestamp, 1*time.Second)
			})
		})

		t.Run("ChannelOperations", func(t *testing.T) {
			t.Run("AddAndGetChannels", func(t *testing.T) {
				// Add channels
				err := storage.AddChannel(channel1)
				assert.NoError(t, err)

				err = storage.AddChannel(channel2)
				assert.NoError(t, err)

				// Get all channels
				channels := storage.Channels()
				assert.Contains(t, channels, channel1)
				assert.Contains(t, channels, channel2)
			})

			t.Run("RemoveChannel", func(t *testing.T) {
				// Remove a channel
				err := storage.RemoveChannel(channel1)
				assert.NoError(t, err)

				// Verify it's gone
				channels := storage.Channels()
				assert.NotContains(t, channels, channel1)

				// Re-add for subsequent tests
				err = storage.AddChannel(channel1)
				assert.NoError(t, err)
			})

			t.Run("ChannelCountOperations", func(t *testing.T) {
				// Initially zero
				count := storage.GetChannelCount(channel1)
				assert.Equal(t, int64(0), count)

				// Adjust count
				newCount, err := storage.AdjustChannelCount(node1, channel1, 5)
				assert.NoError(t, err)
				assert.Equal(t, int64(5), newCount)

				// Get updated count
				count = storage.GetChannelCount(channel1)
				assert.Equal(t, int64(5), count)

				// Adjust again
				newCount, err = storage.AdjustChannelCount(node1, channel1, -2)
				assert.NoError(t, err)
				assert.Equal(t, int64(3), newCount)

				// Verify
				count = storage.GetChannelCount(channel1)
				assert.Equal(t, int64(3), count)
			})
		})

		t.Run("PresenceOperations", func(t *testing.T) {
			// Add presence channel first
			err := storage.AddChannel(presenceChannel)
			assert.NoError(t, err)

			t.Run("AddUserToPresence", func(t *testing.T) {
				err = storage.AddUserToPresence(node1, presenceChannel, socket1, memberData1)
				assert.NoError(t, err)

				err = storage.AddUserToPresence(node1, presenceChannel, socket2, memberData2)
				assert.NoError(t, err)

				// Verify data when no user is specified
				presenceData, socketIDs, err := storage.GetPresenceData(presenceChannel, pusherClient.MemberData{})
				assert.NoError(t, err)
				assert.NotEmpty(t, presenceData)
				assert.Len(t, socketIDs, 0)

				// Verify presence data for user 2
				presenceData, socketIDs, err = storage.GetPresenceData(presenceChannel, memberData2)
				assert.NoError(t, err)
				assert.NotEmpty(t, presenceData)
				assert.Len(t, socketIDs, 1)

				// Add another connection for user 2, on a new socket (socket3)
				err = storage.AddUserToPresence(node1, presenceChannel, socket3, memberData2)
				assert.NoError(t, err)

				// Verify data with additional connection for user2
				presenceData, socketIDs, err = storage.GetPresenceData(presenceChannel, memberData2)
				assert.NoError(t, err)
				assert.NotEmpty(t, presenceData)
				assert.Len(t, socketIDs, 2)

			})

			t.Run("GetPresenceDataForSocket", func(t *testing.T) {
				data, err := storage.GetPresenceDataForSocket(node1, presenceChannel, socket1)
				assert.NoError(t, err)
				assert.NotNil(t, data)
				assert.Equal(t, "user1", data.UserID)

				// Test with invalid socket
				data, err = storage.GetPresenceDataForSocket(node1, presenceChannel, "invalid-socket")
				assert.Error(t, err)
				assert.Nil(t, data)
			})

			t.Run("PresenceChannelUserIDs", func(t *testing.T) {
				userIDs := PresenceChannelUserIDs(storage, presenceChannel)
				assert.Contains(t, userIDs, "user1")
				assert.Contains(t, userIDs, "user2")
				assert.Len(t, userIDs, 2)
			})

			t.Run("GetPresenceSocketsForUserID", func(t *testing.T) {
				sockets := GetPresenceSocketsForUserID(storage, presenceChannel, "user1")
				assert.Contains(t, sockets, socket1)
				assert.Len(t, sockets, 1)
			})

			t.Run("RemoveUserFromPresence", func(t *testing.T) {
				err := storage.RemoveUserFromPresence(node1, presenceChannel, socket1)
				assert.NoError(t, err)

				// Verify user is removed
				data, err := storage.GetPresenceDataForSocket(node1, presenceChannel, socket1)
				assert.Error(t, err)
				assert.Nil(t, data)

				// But user2 still exists
				data, err = storage.GetPresenceDataForSocket(node1, presenceChannel, socket2)
				assert.NoError(t, err)
				assert.NotNil(t, data)
			})
		})

		t.Run("SocketHeartbeat", func(t *testing.T) {
			channels := map[constants.ChannelName]constants.ChannelName{
				channel1: channel1,
				channel2: channel2,
			}

			err := storage.SocketDidHeartbeat(node1, socket1, channels)
			assert.NoError(t, err)
		})

		t.Run("PurgeNodeData", func(t *testing.T) {
			// Set up some data for node2
			err := storage.AddNewNode(node2)
			assert.NoError(t, err)

			// Add channel and adjust count
			err = storage.AddChannel(channel2)
			assert.NoError(t, err)

			_, err = storage.AdjustChannelCount(node2, channel2, 3)
			assert.NoError(t, err)

			// Add presence data
			err = storage.AddUserToPresence(node2, presenceChannel, "socket-node2", memberData1)
			assert.NoError(t, err)

			// Now purge the node
			err = storage.PurgeNodeData(node2)
			assert.NoError(t, err)

			// Verify node data is gone
			nodes, err := storage.GetAllNodes()
			assert.NoError(t, err)
			assert.NotContains(t, nodes, string(node2))
		})

		t.Run("Cleanup", func(t *testing.T) {
			// This is mostly a no-op test since Cleanup() doesn't return anything
			// Just ensure it doesn't panic
			storage.Cleanup()
			assert.True(t, true) // If we get here, no panic occurred
		})

		t.Run("UnmarshalPresenceData", func(t *testing.T) {
			// Create presence data JSON
			presenceJSON := `{
				"presence": {
					"ids": ["user1", "user2"],
					"hash": {
						"user1": {"name": "Test User 1"},
						"user2": {"name": "Test User 2"}
					},
					"count": 2
				}
			}`

			data, err := UnmarshalPresenceData([]byte(presenceJSON))
			assert.NoError(t, err)
			assert.Equal(t, 2, data.Count)
			assert.Contains(t, data.IDs, "user1")
			assert.Contains(t, data.IDs, "user2")
			assert.Contains(t, data.Hash, "user1")
			assert.Contains(t, data.Hash, "user2")
		})
	})
}
