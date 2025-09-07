package internal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/metrics"
)

// MockHorizontalInterface for testing
type MockHorizontalInterface struct {
	channelName    string
	numSub         int64
	broadcastCalls []BroadcastCall
}

type BroadcastCall struct {
	Channel string
	Data    string
}

func NewMockHorizontalInterface(channelName string, numSub int64) *MockHorizontalInterface {
	return &MockHorizontalInterface{
		channelName:    channelName,
		numSub:         numSub,
		broadcastCalls: make([]BroadcastCall, 0),
	}
}

func (m *MockHorizontalInterface) GetChannelName() string {
	return m.channelName
}

func (m *MockHorizontalInterface) broadcastToChannel(channel string, data string) {
	m.broadcastCalls = append(m.broadcastCalls, BroadcastCall{
		Channel: channel,
		Data:    data,
	})
}

func (m *MockHorizontalInterface) getNumSub() (int64, error) {
	return m.numSub, nil
}

func (m *MockHorizontalInterface) GetBroadcastCalls() []BroadcastCall {
	return m.broadcastCalls
}

func (m *MockHorizontalInterface) ResetBroadcastCalls() {
	m.broadcastCalls = make([]BroadcastCall, 0)
}

// Helper function to create a test app
func createTestAppForHorizontal() *apps.App {
	app := &apps.App{
		ID:      "test-app",
		Key:     "test-key",
		Secret:  "test-secret",
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test WebSocket
func createTestWebSocketForHorizontal(id constants.SocketID) *WebSocket {
	return &WebSocket{
		ID: id,
		// Add other required fields as needed
	}
}

// Helper function to create a test metrics manager
func createTestMetricsManagerForHorizontal() metrics.MetricsInterface {
	return &metrics.NoOpMetrics{}
}

func TestNewHorizontalAdapter(t *testing.T) {
	t.Run("SuccessfulCreation", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()

		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, mockInterface, adapter.concreteAdapter)
		assert.Equal(t, metricsManager, adapter.metricsManager)
		assert.Equal(t, "horizontal-adapter", adapter.channel)
		assert.NotEmpty(t, adapter.uuid)
		assert.Equal(t, 5, adapter.requestTimeout)
	})

	t.Run("NilMetricsManager", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)

		adapter, err := NewHorizontalAdapter(mockInterface, nil)

		// Should still work as LocalAdapter.Init() doesn't require metrics
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})

	t.Run("NilInterface", func(t *testing.T) {
		metricsManager := createTestMetricsManagerForHorizontal()

		adapter, err := NewHorizontalAdapter(nil, metricsManager)

		// This should fail because we can't call methods on nil interface
		assert.Error(t, err)
		assert.Nil(t, adapter)
	})
}

func TestHorizontalAdapter_Send(t *testing.T) {
	t.Run("SendWithExceptingIds", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		data := []byte("test-data")
		exceptingIds := []constants.SocketID{"socket1", "socket2"}

		err = adapter.Send(appID, channel, data, exceptingIds...)

		assert.NoError(t, err)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		assert.Equal(t, "test-channel", broadcastCall.Channel)

		// Verify the broadcast data contains the expected fields
		var broadcastData AdapterMessageToSend
		err = json.Unmarshal([]byte(broadcastCall.Data), &broadcastData)
		assert.NoError(t, err)
		assert.Equal(t, adapter.uuid, broadcastData.Uuid)
		assert.Equal(t, appID, broadcastData.AppID)
		assert.Equal(t, channel, broadcastData.Channel)
		assert.Equal(t, "test-data", broadcastData.Data)
		assert.Equal(t, "socket1", broadcastData.ExceptingId) // Should take first excepting ID
	})

	t.Run("SendWithoutExceptingIds", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		data := []byte("test-data")

		err = adapter.Send(appID, channel, data)

		assert.NoError(t, err)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var broadcastData AdapterMessageToSend
		err = json.Unmarshal([]byte(broadcastCall.Data), &broadcastData)
		assert.NoError(t, err)
		assert.Empty(t, broadcastData.ExceptingId)
	})

	t.Run("SendEmptyData", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		data := []byte("")

		err = adapter.Send(appID, channel, data)

		assert.NoError(t, err)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)
	})
}

func TestHorizontalAdapter_TerminateUserConnections(t *testing.T) {
	t.Run("TerminateUserConnections", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		userID := "user123"

		adapter.TerminateUserConnections(appID, userID)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, TERMINATE_USER_CONNECTIONS, requestBody.Type)
		assert.Equal(t, appID, requestBody.AppID)
		assert.Equal(t, "user123", requestBody.Opts["userId"])
	})
}

func TestHorizontalAdapter_GetSockets(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		sockets := adapter.GetSockets(appID, true)

		// Should return local sockets only, no broadcast calls
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithSingleSubscriber", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 1)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		sockets := adapter.GetSockets(appID, false)

		// Should return local sockets only when numSub <= 1
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetSockets(appID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, SOCKETS, requestBody.Type)
		assert.Equal(t, appID, requestBody.AppID)
	})
}

func TestHorizontalAdapter_GetSocketsCount(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		count := adapter.GetSocketsCount(appID, true)

		// Should return local count only
		assert.GreaterOrEqual(t, count, int64(0))
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetSocketsCount(appID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, SOCKETS_COUNT, requestBody.Type)
	})
}

func TestHorizontalAdapter_GetChannels(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		adapter.GetChannels(appID, true)

		// Should return local channels only (may be empty map or nil)
		// Note: GetChannels may return nil if no local channels exist
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannels(appID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNELS, requestBody.Type)
	})
}

func TestHorizontalAdapter_GetChannelSockets(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		sockets := adapter.GetChannelSockets(appID, channel, true)

		// Should return local channel sockets only
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannelSockets(appID, channel, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNEL_SOCKETS, requestBody.Type)
		assert.Equal(t, "test-channel", requestBody.Opts["channel"])
	})
}

func TestHorizontalAdapter_GetChannelSocketsCount(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		count := adapter.GetChannelSocketsCount(appID, channel, true)

		// Should return local count only
		assert.GreaterOrEqual(t, count, int64(0))
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannelSocketsCount(appID, channel, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNEL_SOCKETS_COUNT, requestBody.Type)
		assert.Equal(t, "test-channel", requestBody.Opts["channel"])
	})
}

func TestHorizontalAdapter_GetChannelMembers(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		members := adapter.GetChannelMembers(appID, channel, true)

		// Should return local members only
		assert.NotNil(t, members)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannelMembers(appID, channel, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNEL_MEMBERS, requestBody.Type)
		assert.Equal(t, "test-channel", requestBody.Opts["channel"])
	})
}

func TestHorizontalAdapter_GetChannelMembersCount(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		count := adapter.GetChannelMembersCount(appID, channel, true)

		// Should return local count only
		assert.GreaterOrEqual(t, count, 0)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannelMembersCount(appID, channel, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNEL_MEMBERS_COUNT, requestBody.Type)
		assert.Equal(t, "test-channel", requestBody.Opts["channel"])
	})
}

func TestHorizontalAdapter_IsInChannel(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		socketID := constants.SocketID("socket1")
		exists := adapter.IsInChannel(appID, channel, socketID, true)

		// Should return local result only
		assert.False(t, exists) // Assuming no local sockets
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		socketID := constants.SocketID("socket1")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.IsInChannel(appID, channel, socketID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, SOCKET_EXISTS_IN_CHANNEL, requestBody.Type)
		assert.Equal(t, "test-channel", requestBody.Opts["channel"])
		assert.Equal(t, "socket1", requestBody.Opts["socketId"])
	})
}

func TestHorizontalAdapter_GetPresenceChannelsWithUsersCount(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		counts := adapter.GetPresenceChannelsWithUsersCount(appID, true)

		// Should return local counts only
		assert.NotNil(t, counts)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetPresenceChannelsWithUsersCount(appID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, PRESENCE_CHANNELS_WITH_USERS_COUNT, requestBody.Type)
	})
}

func TestHorizontalAdapter_GetChannelsWithSocketsCount(t *testing.T) {
	t.Run("OnlyLocalTrue", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		counts := adapter.GetChannelsWithSocketsCount(appID, true)

		// Should return local counts only
		assert.NotNil(t, counts)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("OnlyLocalFalseWithMultipleSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// Test that the method sends a request (we'll check the broadcast call)
		// but we won't wait for the response since it will timeout
		go func() {
			adapter.GetChannelsWithSocketsCount(appID, false)
		}()

		// Give it a moment to send the request
		time.Sleep(100 * time.Millisecond)

		// Should have sent a request
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)

		broadcastCall := mockInterface.GetBroadcastCalls()[0]
		var requestBody HorizontalRequestBody
		err = json.Unmarshal([]byte(broadcastCall.Data), &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, CHANNELS_WITH_SOCKETS_COUNT, requestBody.Type)
	})
}

func TestHorizontalRequestBody_ToJson(t *testing.T) {
	t.Run("ValidRequestBody", func(t *testing.T) {
		requestBody := &HorizontalRequestBody{
			Type:      SOCKETS,
			RequestID: "test-request-id",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}

		jsonData := requestBody.ToJson()

		assert.NotEmpty(t, jsonData)

		// Verify it can be unmarshaled back
		var unmarshaled HorizontalRequestBody
		err := json.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, requestBody.Type, unmarshaled.Type)
		assert.Equal(t, requestBody.RequestID, unmarshaled.RequestID)
		assert.Equal(t, requestBody.AppID, unmarshaled.AppID)
		assert.Equal(t, requestBody.Opts, unmarshaled.Opts)
	})

	t.Run("EmptyRequestBody", func(t *testing.T) {
		requestBody := &HorizontalRequestBody{}

		jsonData := requestBody.ToJson()

		assert.NotEmpty(t, jsonData)

		// Verify it can be unmarshaled back
		var unmarshaled HorizontalRequestBody
		err := json.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, requestBody.Type, unmarshaled.Type)
		assert.Equal(t, requestBody.RequestID, unmarshaled.RequestID)
		assert.Equal(t, requestBody.AppID, unmarshaled.AppID)
	})
}

func TestHorizontalAdapter_EdgeCases(t *testing.T) {
	t.Run("NilConcreteAdapter", func(t *testing.T) {
		metricsManager := createTestMetricsManagerForHorizontal()

		// This should fail during creation
		adapter, err := NewHorizontalAdapter(nil, metricsManager)
		assert.Error(t, err)
		assert.Nil(t, adapter)
	})

	t.Run("ZeroNumSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 0)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		sockets := adapter.GetSockets(appID, false)

		// Should return local sockets only when numSub <= 1
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("NegativeNumSubscribers", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", -1)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		sockets := adapter.GetSockets(appID, false)

		// Should return local sockets only when numSub <= 1
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 0)
	})

	t.Run("EmptyChannelName", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")
		data := []byte("test-data")

		err = adapter.Send(appID, channel, data)

		assert.NoError(t, err)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)
	})
}

func TestHorizontalAdapter_Concurrency(t *testing.T) {
	t.Run("ConcurrentSendOperations", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")
		channel := constants.ChannelName("test-channel")

		// Send multiple messages concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(i int) {
				data := []byte("test-data-" + string(rune(i)))
				err := adapter.Send(appID, channel, data)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 10 broadcast calls
		assert.Len(t, mockInterface.GetBroadcastCalls(), 10)
	})

	t.Run("ConcurrentGetOperations", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		appID := constants.AppID("test-app")

		// Perform multiple get operations concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				sockets := adapter.GetSockets(appID, false)
				assert.NotNil(t, sockets)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 10 broadcast calls
		assert.Len(t, mockInterface.GetBroadcastCalls(), 10)
	})
}

func TestHorizontalAdapter_RequestTypes(t *testing.T) {
	t.Run("AllRequestTypes", func(t *testing.T) {
		// Test that all request types are properly defined
		assert.Equal(t, HorizontalRequestType(0), SOCKETS)
		assert.Equal(t, HorizontalRequestType(1), CHANNELS)
		assert.Equal(t, HorizontalRequestType(2), CHANNEL_SOCKETS)
		assert.Equal(t, HorizontalRequestType(3), CHANNEL_MEMBERS)
		assert.Equal(t, HorizontalRequestType(4), SOCKETS_COUNT)
		assert.Equal(t, HorizontalRequestType(5), CHANNEL_MEMBERS_COUNT)
		assert.Equal(t, HorizontalRequestType(6), CHANNEL_SOCKETS_COUNT)
		assert.Equal(t, HorizontalRequestType(7), SOCKET_EXISTS_IN_CHANNEL)
		assert.Equal(t, HorizontalRequestType(8), CHANNELS_WITH_SOCKETS_COUNT)
		assert.Equal(t, HorizontalRequestType(9), TERMINATE_USER_CONNECTIONS)
		assert.Equal(t, HorizontalRequestType(10), PRESENCE_CHANNELS_WITH_USERS_COUNT)
	})
}

func TestHorizontalAdapter_TimeoutBehavior(t *testing.T) {
	t.Run("RequestTimeout", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 3)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout for testing
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		sockets := adapter.GetSockets(appID, false)

		// Should return local sockets when timeout occurs
		assert.NotNil(t, sockets)
		assert.Len(t, mockInterface.GetBroadcastCalls(), 1)
	})
}

func TestHorizontalAdapter_OnRequest(t *testing.T) {
	t.Run("SOCKETS_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      SOCKETS,
			RequestID: "test-request-1",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-1", response.RequestID)
		assert.NotNil(t, response.Sockets)
	})

	t.Run("CHANNEL_SOCKETS_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_SOCKETS,
			RequestID: "test-request-2",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-2", response.RequestID)
		assert.NotNil(t, response.Sockets)
	})

	t.Run("CHANNEL_SOCKETS_Request_NoChannel", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_SOCKETS,
			RequestID: "test-request-3",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when channel is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("CHANNELS_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNELS,
			RequestID: "test-request-4",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-4", response.RequestID)
		// Channels may be nil if no local channels exist
	})

	t.Run("CHANNELS_WITH_SOCKETS_COUNT_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNELS_WITH_SOCKETS_COUNT,
			RequestID: "test-request-5",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-5", response.RequestID)
		assert.NotNil(t, response.ChannelsWithSocketsCount)
	})

	t.Run("CHANNEL_MEMBERS_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_MEMBERS,
			RequestID: "test-request-6",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-6", response.RequestID)
		assert.NotNil(t, response.Members)
	})

	t.Run("CHANNEL_MEMBERS_Request_NoChannel", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_MEMBERS,
			RequestID: "test-request-7",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when channel is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("SOCKETS_COUNT_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      SOCKETS_COUNT,
			RequestID: "test-request-8",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-8", response.RequestID)
		assert.GreaterOrEqual(t, response.TotalCount, int64(0))
	})

	t.Run("CHANNEL_MEMBERS_COUNT_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_MEMBERS_COUNT,
			RequestID: "test-request-9",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-9", response.RequestID)
		assert.GreaterOrEqual(t, response.TotalCount, int64(0))
	})

	t.Run("CHANNEL_MEMBERS_COUNT_Request_NoChannel", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_MEMBERS_COUNT,
			RequestID: "test-request-10",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when channel is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("CHANNEL_SOCKETS_COUNT_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_SOCKETS_COUNT,
			RequestID: "test-request-11",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-11", response.RequestID)
		assert.GreaterOrEqual(t, response.TotalCount, int64(0))
	})

	t.Run("CHANNEL_SOCKETS_COUNT_Request_NoChannel", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      CHANNEL_SOCKETS_COUNT,
			RequestID: "test-request-12",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when channel is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("SOCKET_EXISTS_IN_CHANNEL_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      SOCKET_EXISTS_IN_CHANNEL,
			RequestID: "test-request-13",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel":  "test-channel",
					"socketId": "socket-123",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-13", response.RequestID)
		assert.False(t, response.Exists) // Assuming no local sockets
	})

	t.Run("SOCKET_EXISTS_IN_CHANNEL_Request_NoChannel", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      SOCKET_EXISTS_IN_CHANNEL,
			RequestID: "test-request-14",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"socketId": "socket-123",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when channel is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("SOCKET_EXISTS_IN_CHANNEL_Request_NoSocketId", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      SOCKET_EXISTS_IN_CHANNEL,
			RequestID: "test-request-15",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"channel": "test-channel",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when socketId is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("TERMINATE_USER_CONNECTIONS_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      TERMINATE_USER_CONNECTIONS,
			RequestID: "test-request-16",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{
					"userId": "user-123",
				},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, `{"success": true}`, string(data))
	})

	t.Run("TERMINATE_USER_CONNECTIONS_Request_NoUserId", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      TERMINATE_USER_CONNECTIONS,
			RequestID: "test-request-17",
			AppID:     "test-app",
			HorizontalRequestOptions: HorizontalRequestOptions{
				Opts: map[string]string{},
			},
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil when userId is missing
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("PRESENCE_CHANNELS_WITH_USERS_COUNT_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      PRESENCE_CHANNELS_WITH_USERS_COUNT,
			RequestID: "test-request-18",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		assert.NoError(t, err)
		assert.NotNil(t, data)

		var response HorizontalResponse
		err = json.Unmarshal(data, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-request-18", response.RequestID)
		assert.NotNil(t, response.ChannelsWithSocketsCount)
	})

	t.Run("Unknown_Request_Type", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		requestBody := HorizontalRequestBody{
			Type:      HorizontalRequestType(999), // Unknown type
			RequestID: "test-request-19",
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil for unknown request types
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("Invalid_JSON_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		invalidJSON := `{"invalid": json}`

		data, err := adapter.onRequest("test-channel", invalidJSON)

		// Should return error for invalid JSON
		assert.Error(t, err)
		assert.Nil(t, data)
	})

	t.Run("Duplicate_Request_ID", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Add a request to the requests map first
		requestID := "duplicate-request-id"
		adapter.mutex.Lock()
		adapter.requests[requestID] = &HorizontalRequest{
			requestID: requestID,
		}
		adapter.mutex.Unlock()

		requestBody := HorizontalRequestBody{
			Type:      SOCKETS,
			RequestID: requestID,
			AppID:     "test-app",
		}
		requestJSON, _ := json.Marshal(requestBody)

		data, err := adapter.onRequest("test-channel", string(requestJSON))

		// Should return nil for duplicate request IDs
		assert.NoError(t, err)
		assert.Nil(t, data)
	})
}

func TestHorizontalAdapter_OnResponse(t *testing.T) {
	t.Run("Valid_Response", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Create a request and add it to the requests map
		requestID := "test-response-1"
		request := &HorizontalRequest{
			requestID:          requestID,
			requestResolveChan: make(chan *HorizontalResponse, 1),
		}
		adapter.mutex.Lock()
		adapter.requests[requestID] = request
		adapter.mutex.Unlock()

		response := HorizontalResponse{
			RequestID: requestID,
			Sockets:   make(map[constants.SocketID]*WebSocket),
		}
		responseJSON, _ := json.Marshal(response)

		// Call onResponse in a goroutine to avoid blocking
		go adapter.onResponse("test-channel", string(responseJSON))

		// Wait a moment for the response to be processed
		time.Sleep(100 * time.Millisecond)

		// The response should have been sent to the requestResolveChan
		// We can't easily test this without more complex setup, but we can verify
		// that the method doesn't panic and processes the response
	})

	t.Run("Invalid_JSON_Response", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		invalidJSON := `{"invalid": json}`

		// Should not panic with invalid JSON
		adapter.onResponse("test-channel", invalidJSON)
	})

	t.Run("Response_For_Unknown_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		response := HorizontalResponse{
			RequestID: "unknown-request-id",
			Sockets:   make(map[constants.SocketID]*WebSocket),
		}
		responseJSON, _ := json.Marshal(response)

		// Should not panic for unknown request ID
		adapter.onResponse("test-channel", string(responseJSON))
	})
}

func TestHorizontalAdapter_WaitForResponse(t *testing.T) {
	t.Run("Timeout_Reached", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout
		adapter.requestTimeout = 1

		request := &HorizontalRequest{
			requestID:           "test-wait-1",
			appID:               "test-app",
			requestType:         SOCKETS,
			requestResolveChan:  make(chan *HorizontalResponse, 1),
			requestResponseChan: make(chan *HorizontalResponse, 1),
		}
		request.numSub = 2

		// Start waitForResponse in a goroutine
		go adapter.waitForResponse(request)

		// Wait for timeout
		time.Sleep(2 * time.Second)

		// The method should have completed (we can't easily test the exact behavior
		// without more complex setup, but we can verify it doesn't panic)
	})

	t.Run("Nil_Request", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Should not panic with nil request
		adapter.waitForResponse(nil)
	})

	t.Run("Response_Received", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a longer timeout
		adapter.requestTimeout = 5

		request := &HorizontalRequest{
			requestID:           "test-wait-2",
			appID:               "test-app",
			requestType:         SOCKETS,
			requestResolveChan:  make(chan *HorizontalResponse, 1),
			requestResponseChan: make(chan *HorizontalResponse, 1),
		}
		request.numSub = 2

		// Start waitForResponse in a goroutine
		go adapter.waitForResponse(request)

		// Send a response
		response := &HorizontalResponse{
			RequestID: "test-wait-2",
			Sockets:   make(map[constants.SocketID]*WebSocket),
		}
		request.requestResolveChan <- response

		// Wait a moment for processing
		time.Sleep(100 * time.Millisecond)

		// The method should have processed the response
	})
}

func TestHorizontalAdapter_SendRequest(t *testing.T) {
	t.Run("SendRequest_With_Options", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		requestExtra := &HorizontalRequestExtra{
			numSub: 2,
		}
		requestOptions := &HorizontalRequestOptions{
			Opts: map[string]string{
				"channel": "test-channel",
			},
		}

		// This will timeout, but we can test that it sends the request
		response := adapter.sendRequest(appID, CHANNEL_SOCKETS, requestExtra, requestOptions)

		// Should return a response (even if it's from timeout)
		assert.NotNil(t, response)
		// RequestID will be a generated UUID, not the channel name
		assert.NotEmpty(t, response.RequestID)
	})

	t.Run("SendRequest_Without_Options", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		requestExtra := &HorizontalRequestExtra{
			numSub: 2,
		}

		// This will timeout, but we can test that it sends the request
		response := adapter.sendRequest(appID, SOCKETS, requestExtra, nil)

		// Should return a response (even if it's from timeout)
		assert.NotNil(t, response)
		// RequestID will be a generated UUID, not the channel name
		assert.NotEmpty(t, response.RequestID)
	})
}

func TestHorizontalAdapter_EdgeCases2(t *testing.T) {
	t.Run("Nil_RequestExtra", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")

		// This will timeout, but we can test that it handles nil requestExtra
		response := adapter.sendRequest(appID, SOCKETS, nil, nil)

		// Should return a response (even if it's from timeout)
		assert.NotNil(t, response)
	})

	t.Run("Empty_RequestExtra", func(t *testing.T) {
		mockInterface := NewMockHorizontalInterface("test-channel", 2)
		metricsManager := createTestMetricsManagerForHorizontal()
		adapter, err := NewHorizontalAdapter(mockInterface, metricsManager)
		require.NoError(t, err)

		// Set a very short timeout to avoid hanging
		adapter.requestTimeout = 1

		appID := constants.AppID("test-app")
		requestExtra := &HorizontalRequestExtra{}

		// This will timeout, but we can test that it handles empty requestExtra
		response := adapter.sendRequest(appID, SOCKETS, requestExtra, nil)

		// Should return a response (even if it's from timeout)
		assert.NotNil(t, response)
	})
}
