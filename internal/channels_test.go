package internal

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"

	"github.com/stretchr/testify/assert"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

// MockAdapterForChannels is a mock adapter specifically for testing channels.go
// It extends the basic MockAdapter with configurable behavior for channel operations
type MockAdapterForChannels struct {
	*MockAdapter
	channelMembersCount    int
	channelSocketsCount    int64
	addToChannelError      error
	removeFromChannelCount int64
	presenceData           map[constants.SocketID]*pusherClient.MemberData
	mu                     sync.Mutex
}

func NewMockAdapterForChannels() *MockAdapterForChannels {
	return &MockAdapterForChannels{
		MockAdapter: &MockAdapter{
			namespaces: make(map[constants.AppID]*Namespace),
		},
		channelMembersCount: 0,
		channelSocketsCount: 1,
		presenceData:        make(map[constants.SocketID]*pusherClient.MemberData),
	}
}

func (m *MockAdapterForChannels) GetChannelMembersCount(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) int {
	return m.channelMembersCount
}

func (m *MockAdapterForChannels) GetChannelSocketsCount(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) int64 {
	return m.channelSocketsCount
}

func (m *MockAdapterForChannels) AddToChannel(appID constants.AppID, channelName constants.ChannelName, ws *WebSocket) (int64, error) {

	if m.addToChannelError != nil {
		return 0, m.addToChannelError
	}
	return m.channelSocketsCount, nil
}

func (m *MockAdapterForChannels) RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64 {
	return m.removeFromChannelCount
}

func (m *MockAdapterForChannels) GetChannelMembers(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) map[constants.SocketID]*pusherClient.MemberData {
	return m.presenceData
}

// Helper function to create a test app
func createTestAppForChannels() *apps.App {
	app := &apps.App{
		ID:                           "test-app",
		Key:                          "test-key",
		Secret:                       "test-secret",
		MaxPresenceMembersPerChannel: 10,
		MaxPresenceMemberSizeInKb:    5,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test WebSocket
func createTestWebSocketForChannels() *WebSocket {
	return &WebSocket{
		ID: "test-socket-123",
	}
}

// Helper function to create a test WebSocket with presence data
func createTestWebSocketWithPresenceData() *WebSocket {
	ws := &WebSocket{
		ID: "test-socket-presence",
		PresenceData: map[constants.ChannelName]*pusherClient.MemberData{
			"presence-channel": {
				UserID:   "user-123",
				UserInfo: map[string]string{"name": "Test User"},
			},
		},
	}
	return ws
}

func TestCreateChannelFromString(t *testing.T) {
	app := createTestAppForChannels()

	t.Run("PublicChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")

		assert.Equal(t, app, channel.App)
		assert.Equal(t, constants.ChannelName("public-channel"), channel.Name)
		assert.Equal(t, constants.ChannelTypePublic, channel.Type)
		assert.False(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.False(t, channel.IsCache)
		assert.NotNil(t, channel.Connections)
	})

	t.Run("PublicChannelWithCache", func(t *testing.T) {
		channel := CreateChannelFromString(app, "cache-public-channel")

		assert.Equal(t, constants.ChannelTypePublic, channel.Type)
		assert.False(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.True(t, channel.IsCache)
	})

	t.Run("PrivateChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-channel")

		assert.Equal(t, constants.ChannelTypePrivate, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.False(t, channel.IsCache)
	})

	t.Run("PrivateChannelWithCache", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-cache-channel")

		assert.Equal(t, constants.ChannelTypePrivate, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.True(t, channel.IsCache)
	})

	t.Run("PrivateEncryptedChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-encrypted-channel")

		assert.Equal(t, constants.ChannelTypePrivateEncrypted, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.True(t, channel.IsEncrypted)
		assert.False(t, channel.IsCache)
	})

	t.Run("PrivateEncryptedChannelWithCache", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-encrypted-cache-channel")

		assert.Equal(t, constants.ChannelTypePrivateEncrypted, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.True(t, channel.IsEncrypted)
		assert.True(t, channel.IsCache)
	})

	t.Run("PresenceChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")

		assert.Equal(t, constants.ChannelTypePresence, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.False(t, channel.IsCache)
	})

	t.Run("PresenceChannelWithCache", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-cache-channel")

		assert.Equal(t, constants.ChannelTypePresence, channel.Type)
		assert.True(t, channel.RequiresAuth)
		assert.False(t, channel.IsEncrypted)
		assert.True(t, channel.IsCache)
	})
}

func TestChannel_String(t *testing.T) {
	app := createTestAppForChannels()
	channel := CreateChannelFromString(app, "test-channel")

	assert.Equal(t, "test-channel", channel.String())
}

func TestChannel_Join(t *testing.T) {
	app := createTestAppForChannels()
	adapter := NewMockAdapterForChannels()
	ws := createTestWebSocketForChannels()

	t.Run("PublicChannel_NoAuth", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.True(t, response.Success)
		assert.Equal(t, int64(1), response.ChannelConnections)
		assert.Nil(t, response.Member)
	})

	t.Run("PrivateChannel_ValidAuth", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-channel")

		// Create valid auth signature in the format expected by ValidateChannelAuth
		reconstructedString := string(ws.ID) + ":" + string(channel.Name)
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "private-channel",
				Auth:    auth,
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.True(t, response.Success)
		assert.Equal(t, int64(1), response.ChannelConnections)
		assert.Nil(t, response.Member)
	})

	t.Run("PrivateChannel_InvalidAuth", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-channel")
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "private-channel",
				Auth:    "invalid-auth",
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
		assert.Equal(t, util.ErrCodeSubscriptionAccessDenied, response.ErrorCode)
		assert.Equal(t, "Invalid signature", response.Message)
		assert.Equal(t, "AuthError", response.Type)
	})

	t.Run("PresenceChannel_ValidAuth", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")

		// Create valid auth signature in the format expected by ValidateChannelAuth
		memberData := `{"user_id": "user-123", "user_info": {"name": "Test User"}}`
		reconstructedString := string(ws.ID) + ":" + string(channel.Name) + ":" + memberData
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: memberData,
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.True(t, response.Success)
		assert.Equal(t, int64(1), response.ChannelConnections)
		assert.NotNil(t, response.Member)
		assert.Equal(t, "user-123", response.Member.UserID)
	})

	t.Run("PresenceChannel_ExceedsMemberLimit", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")
		adapter.channelMembersCount = 10 // At the limit

		memberData := `{"user_id": "user-123", "user_info": {"name": "Test User"}}`
		reconstructedString := string(ws.ID) + ":" + string(channel.Name) + ":" + memberData
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: memberData,
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
		assert.Equal(t, util.ErrCodeOverCapacity, response.ErrorCode)
		assert.Equal(t, "The maximum number of members in this Channel has been reached", response.Message)
		assert.Equal(t, "LimitReached", response.Type)
	})

	t.Run("PresenceChannel_MemberDataTooLarge", func(t *testing.T) {
		// Create app with very small member size limit
		app := &apps.App{
			ID:                           "test-app",
			Key:                          "test-key",
			Secret:                       "test-secret",
			MaxPresenceMembersPerChannel: 10,
			MaxPresenceMemberSizeInKb:    1, // Very small limit
		}
		app.SetMissingDefaults()

		channel := CreateChannelFromString(app, "presence-channel-data-too-large")
		// increase the member limit, because there are already members on the adapter,
		// and we need to ensure we don't get stopped at the member count check
		// since this mock adapter doesn't actually track per-channel connections
		channel.App.MaxPresenceMembersPerChannel = 1000

		// Create large member data (over 1KB)
		largeUserInfo := make(map[string]string, 200)
		for i := 0; i < 200; i++ {
			largeUserInfo[fmt.Sprintf("field%d", i)] = "this is a very long string that will make the data exceed 1KB limit"
		}
		memberDataBytes, _ := json.Marshal(pusherClient.MemberData{
			UserID:   "user-123",
			UserInfo: largeUserInfo,
		})
		memberData := string(memberDataBytes)

		reconstructedString := string(ws.ID) + ":" + string(channel.Name) + ":" + memberData
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: memberData,
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
		assert.Equal(t, util.ErrCodeClientEventRejected, response.ErrorCode)
		assert.Contains(t, response.Message, "The maximum size of the member data is 1 KB")
		assert.Equal(t, "LimitReached", response.Type)
	})

	t.Run("PresenceChannel_AddToChannelError", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel-add-error")
		adapter.addToChannelError = assert.AnError // force an error when attempting to join channel

		memberData := `{"user_id": "user-123", "user_info": {"name": "Test User"}}`
		reconstructedString := string(ws.ID) + ":" + string(channel.Name) + ":" + memberData
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: memberData,
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
	})

	t.Run("NonPresenceChannel_AddToChannelError", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")
		adapter.addToChannelError = assert.AnError

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
	})
}

func TestChannel_Leave(t *testing.T) {
	app := createTestAppForChannels()
	adapter := NewMockAdapterForChannels()
	adapter.channelSocketsCount = 5
	adapter.removeFromChannelCount = 1

	t.Run("PublicChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")
		ws := createTestWebSocketForChannels()

		response := channel.Leave(adapter, ws)

		assert.True(t, response.Success)
		assert.Equal(t, int64(5), response.RemainingConnections)
		assert.Nil(t, response.Member)
	})

	t.Run("PresenceChannel_WithMemberData", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")
		ws := createTestWebSocketWithPresenceData()

		response := channel.Leave(adapter, ws)

		assert.True(t, response.Success)
		assert.Equal(t, int64(5), response.RemainingConnections)
		assert.NotNil(t, response.Member)
		assert.Equal(t, "user-123", response.Member.UserID)
	})

	t.Run("PresenceChannel_NoMemberData", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")
		ws := createTestWebSocketForChannels()

		response := channel.Leave(adapter, ws)

		assert.True(t, response.Success)
		assert.Equal(t, int64(5), response.RemainingConnections)
		assert.Nil(t, response.Member)
	})

	t.Run("PrivateChannel", func(t *testing.T) {
		channel := CreateChannelFromString(app, "private-channel")
		ws := createTestWebSocketForChannels()

		response := channel.Leave(adapter, ws)

		assert.True(t, response.Success)
		assert.Equal(t, int64(5), response.RemainingConnections)
		assert.Nil(t, response.Member)
	})
}

func TestChannel_EdgeCases(t *testing.T) {
	app := createTestAppForChannels()
	adapter := NewMockAdapterForChannels()

	t.Run("EmptyChannelName", func(t *testing.T) {
		channel := CreateChannelFromString(app, "")

		assert.Equal(t, constants.ChannelTypePublic, channel.Type) // Default type
		assert.Equal(t, constants.ChannelName(""), channel.Name)
	})

	t.Run("ChannelWithSpecialCharacters", func(t *testing.T) {
		channel := CreateChannelFromString(app, "channel-with-special-chars-!@#$%")

		assert.Equal(t, constants.ChannelTypePublic, channel.Type)
		assert.Equal(t, constants.ChannelName("channel-with-special-chars-!@#$%"), channel.Name)
	})

	t.Run("VeryLongChannelName", func(t *testing.T) {
		longName := "very-long-channel-name-" + string(make([]byte, 200))
		channel := CreateChannelFromString(app, constants.ChannelName(longName))

		assert.Equal(t, constants.ChannelTypePublic, channel.Type)
		assert.Equal(t, constants.ChannelName(longName), channel.Name)
	})

	t.Run("ChannelWithMultiplePrefixes", func(t *testing.T) {
		// Test that the first matching prefix is used
		channel := CreateChannelFromString(app, "presence-private-channel")

		assert.Equal(t, constants.ChannelTypePresence, channel.Type)
		assert.True(t, channel.RequiresAuth)
	})

	t.Run("InvalidMemberDataJSON", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")
		ws := createTestWebSocketForChannels()

		auth := util.HmacSignature(string(ws.ID)+":"+string(channel.Name), app.Secret)
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: "invalid-json",
			},
		}

		response := channel.Join(adapter, ws, message)

		assert.False(t, response.Success)
		assert.Nil(t, response.Member)
	})

	t.Run("NilWebSocket", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}

		// This should not panic
		response := channel.Join(adapter, nil, message)
		assert.True(t, response.Success)
	})

	t.Run("NilAdapter", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")
		ws := createTestWebSocketForChannels()
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}

		response := channel.Join(nil, ws, message)
		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "No adapter provided")
	})
}

func TestChannel_Integration(t *testing.T) {
	app := createTestAppForChannels()
	adapter := NewMockAdapterForChannels()
	ws := createTestWebSocketForChannels()

	t.Run("FullJoinLeaveCycle", func(t *testing.T) {
		channel := CreateChannelFromString(app, "public-channel")

		// Join
		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}
		joinResponse := channel.Join(adapter, ws, message)
		assert.True(t, joinResponse.Success)

		// Leave
		leaveResponse := channel.Leave(adapter, ws)
		assert.True(t, leaveResponse.Success)
	})

	t.Run("PresenceChannelFullCycle", func(t *testing.T) {
		channel := CreateChannelFromString(app, "presence-channel")
		ws := createTestWebSocketWithPresenceData()

		// Join
		memberData := `{"user_id": "user-123", "user_info": {"name": "Test User"}}`
		reconstructedString := string(ws.ID) + ":" + string(channel.Name) + ":" + memberData
		expected := util.HmacSignature(reconstructedString, app.Secret)
		auth := "key:" + expected

		message := payloads.SubscribePayload{
			Data: payloads.SubscribeChannelData{
				Channel:     "presence-channel",
				Auth:        auth,
				ChannelData: memberData,
			},
		}
		joinResponse := channel.Join(adapter, ws, message)
		assert.True(t, joinResponse.Success)
		assert.NotNil(t, joinResponse.Member)

		// Leave
		leaveResponse := channel.Leave(adapter, ws)
		assert.True(t, leaveResponse.Success)
		assert.NotNil(t, leaveResponse.Member)
	})
}

func TestChannel_ConcurrentAccess(t *testing.T) {
	app := createTestAppForChannels()
	adapter := NewMockAdapterForChannels()
	channel := CreateChannelFromString(app, "public-channel")

	t.Run("ConcurrentJoins", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				ws := &WebSocket{ID: constants.SocketID(fmt.Sprintf("socket-%d", id))}
				message := payloads.SubscribePayload{
					Data: payloads.SubscribeChannelData{
						Channel: "public-channel",
					},
				}
				response := channel.Join(adapter, ws, message)
				assert.True(t, response.Success)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}
