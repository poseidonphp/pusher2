package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pusher/internal/apps"
	"pusher/internal/cache"
	"pusher/internal/constants"
	"pusher/internal/metrics"

	"github.com/gin-gonic/gin"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

// MockAdapterForApiChannels is a mock adapter specifically for testing apiChannels.go
type MockAdapterForApiChannels struct {
	*MockAdapter
	channelsWithSocketsCount       map[constants.ChannelName]int64
	presenceChannelsWithUsersCount map[constants.ChannelName]int64
	channelMembersCount            int
	channelSocketsCount            int64
	channelMembers                 map[constants.SocketID]*pusherClient.MemberData
}

func NewMockAdapterForApiChannels() *MockAdapterForApiChannels {
	return &MockAdapterForApiChannels{
		MockAdapter: &MockAdapter{
			namespaces: make(map[constants.AppID]*Namespace),
		},
		channelsWithSocketsCount:       make(map[constants.ChannelName]int64),
		presenceChannelsWithUsersCount: make(map[constants.ChannelName]int64),
		channelMembersCount:            0,
		channelSocketsCount:            0,
		channelMembers:                 make(map[constants.SocketID]*pusherClient.MemberData),
	}
}

func (m *MockAdapterForApiChannels) GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return m.channelsWithSocketsCount
}

func (m *MockAdapterForApiChannels) GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return m.presenceChannelsWithUsersCount
}

func (m *MockAdapterForApiChannels) GetChannelMembersCount(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) int {
	return m.channelMembersCount
}

func (m *MockAdapterForApiChannels) GetChannelSocketsCount(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) int64 {
	return m.channelSocketsCount
}

func (m *MockAdapterForApiChannels) GetChannelMembers(appID constants.AppID, channelName constants.ChannelName, onlyLocal bool) map[constants.SocketID]*pusherClient.MemberData {
	return m.channelMembers
}

func (m *MockAdapterForApiChannels) TerminateUserConnections(appID constants.AppID, userID string) {
	// Mock implementation - just track that it was called
}

// MockCacheManagerForApiChannels is a mock cache manager for testing
// type MockCacheManagerForApiChannels struct {
// 	cacheData map[string]string
// }
//
// func NewMockCacheManagerForApiChannels() *MockCacheManagerForApiChannels {
// 	return &MockCacheManagerForApiChannels{
// 		cacheData: make(map[string]string),
// 	}
// }
//
// func (m *MockCacheManagerForApiChannels) Get(key string) (string, bool) {
// 	if value, exists := m.cacheData[key]; exists {
// 		return value, true
// 	}
// 	return "", false
// }
//
// func (m *MockCacheManagerForApiChannels) Set(key string, value string) {
// 	m.cacheData[key] = value
// }
//
// func (m *MockCacheManagerForApiChannels) SetEx(key string, value string, ttl time.Duration) {
// 	m.cacheData[key] = value
// }
//
// func (m *MockCacheManagerForApiChannels) Del(key string) error {
// 	delete(m.cacheData, key)
// 	return nil
// }
//
// func (m *MockCacheManagerForApiChannels) Delete(key string) {
// 	delete(m.cacheData, key)
// }
//
// func (m *MockCacheManagerForApiChannels) Init() error {
// 	return nil
// }
//
// func (m *MockCacheManagerForApiChannels) Remember(key string, ttl int, callback func() (string, error)) (string, error) {
// 	return callback()
// }
//
// func (m *MockCacheManagerForApiChannels) Has(key string) bool {
// 	_, exists := m.cacheData[key]
// 	return exists
// }
//
// func (m *MockCacheManagerForApiChannels) Update(key string, value string) {
// 	m.cacheData[key] = value
// }

// Helper function to create a test app for API channels
func createTestAppForApiChannels() *apps.App {
	app := &apps.App{
		ID:     "test-app",
		Key:    "test-key",
		Secret: "test-secret",
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test server for API channels
func createTestServerForApiChannels() (*Server, *MockAdapterForApiChannels, cache.CacheContract, context.CancelFunc) {
	adapter := NewMockAdapterForApiChannels()
	cacheManager := &cache.LocalCache{}
	metricsManager := &metrics.NoOpMetrics{}

	ctx, cancel := context.WithCancel(context.Background())
	cacheManager.Init(ctx)

	server := &Server{
		Adapter:        adapter,
		CacheManager:   cacheManager,
		MetricsManager: metricsManager,
	}

	return server, adapter, cacheManager, cancel
}

func TestChannelIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("BasicChannelList", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up mock data
		adapter.channelsWithSocketsCount = map[constants.ChannelName]int64{
			"public-channel-1":  5,
			"public-channel-2":  3,
			"private-channel-1": 2,
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.ChannelsList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Channels, 3)
		assert.Contains(t, response.Channels, "public-channel-1")
		assert.Contains(t, response.Channels, "public-channel-2")
		assert.Contains(t, response.Channels, "private-channel-1")
	})

	t.Run("FilterByPrefix", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up mock data
		adapter.channelsWithSocketsCount = map[constants.ChannelName]int64{
			"public-channel-1":   5,
			"private-channel-1":  2,
			"presence-channel-1": 3,
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels?filter_by_prefix=public-", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.ChannelsList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Channels, 1)
		assert.Contains(t, response.Channels, "public-channel-1")
		assert.NotContains(t, response.Channels, "private-channel-1")
		assert.NotContains(t, response.Channels, "presence-channel-1")
	})

	t.Run("PresenceChannelsWithUserCount", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up mock data for presence channels
		adapter.presenceChannelsWithUsersCount = map[constants.ChannelName]int64{
			"presence-channel-1": 5,
			"presence-channel-2": 3,
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels?filter_by_prefix=presence-&info=user_count", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.ChannelsList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Channels, 2)
		assert.Equal(t, 5, response.Channels["presence-channel-1"].UserCount)
		assert.Equal(t, 3, response.Channels["presence-channel-2"].UserCount)
	})

	t.Run("UserCountWithoutPresenceFilter", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()
		app := createTestAppForApiChannels()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels?info=user_count", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "info=user_count requires filter_by_prefix for presence channels", response["error"])
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("GET", "/channels", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})

	t.Run("EmptyChannelsList", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up empty mock data
		adapter.channelsWithSocketsCount = map[constants.ChannelName]int64{}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.ChannelsList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Channels, 0)
	})
}

func TestChannelShow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("BasicChannelInfo", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelSocketsCount = 5

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/public-channel", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "public-channel", response["Name"])
	})

	t.Run("PresenceChannelWithUserCount", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelMembersCount = 3

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/presence-channel?info=user_count", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "presence-channel", response["Name"])
		assert.Equal(t, float64(3), response["user_count"])
		assert.Equal(t, true, response["occupied"])
	})

	t.Run("ChannelWithSubscriptionCount", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelSocketsCount = 7

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/public-channel?info=subscription_count", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "public-channel", response["Name"])
		assert.Equal(t, float64(7), response["subscription_count"])
		assert.Equal(t, true, response["occupied"])
	})

	t.Run("CacheChannelWithCacheInfo", func(t *testing.T) {
		server, _, cacheManager, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up cache data
		cacheKey := cache.GetChannelCacheKey(app.ID, "cache-channel")
		cacheManager.Set(cacheKey, "cached-data")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/cache-channel?info=cache", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "cache-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "cache-channel", response["Name"])
		assert.Equal(t, "cached-data", response["cache"])
	})

	t.Run("MultipleInfoFields", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelMembersCount = 4
		adapter.channelSocketsCount = 6

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/presence-channel?info=user_count,subscription_count", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "presence-channel", response["Name"])
		assert.Equal(t, float64(4), response["user_count"])
		assert.Equal(t, float64(6), response["subscription_count"])
		assert.Equal(t, true, response["occupied"])
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("GET", "/channels/public-channel", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})

	t.Run("NonPresenceChannelWithUserCount", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelMembersCount = 3

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/public-channel?info=user_count", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "public-channel", response["Name"])
		// user_count should not be present for non-presence channels
		assert.Nil(t, response["user_count"])
	})
}

func TestChannelUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("PresenceChannelUsers", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up mock members
		adapter.channelMembers = map[constants.SocketID]*pusherClient.MemberData{
			"socket-1": {UserID: "user-1", UserInfo: map[string]string{"name": "User 1"}},
			"socket-2": {UserID: "user-2", UserInfo: map[string]string{"name": "User 2"}},
			"socket-3": {UserID: "user-3", UserInfo: map[string]string{"name": "User 3"}},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/presence-channel/users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.Users
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.List, 3)

		// Check that all user IDs are present
		userIDs := make(map[string]bool)
		for _, user := range response.List {
			userIDs[user.ID] = true
		}
		assert.True(t, userIDs["socket-1"])
		assert.True(t, userIDs["socket-2"])
		assert.True(t, userIDs["socket-3"])
	})

	t.Run("NonPresenceChannel", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/public-channel/users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "presence only", response["error"])
	})

	t.Run("EmptyChannelName", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels//users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: ""}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Channel name required", response["error"])
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("GET", "/channels/presence-channel/users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})

	t.Run("EmptyMembersList", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up empty members
		adapter.channelMembers = map[constants.SocketID]*pusherClient.MemberData{}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/presence-channel/users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.Users
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.List, 0)
	})
}

func TestApiChannels_TerminateUserConnections(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SuccessfulTermination", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("POST", "/users/user-123/terminate_connections", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "user_id", Value: "user-123"}}

		TerminateUserConnections(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Empty(t, response) // Should be empty JSON object
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("POST", "/users//terminate_connections", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "user_id", Value: ""}}

		TerminateUserConnections(c, server)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Channel name required", response["error"])
	})

	t.Run("NoAppInContext", func(t *testing.T) {
		server, _, _, cancel := createTestServerForApiChannels()
		defer cancel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest("POST", "/users/user-123/terminate_connections", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "user_id", Value: "user-123"}}

		TerminateUserConnections(c, server)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "App not found in context", response["error"])
	})
}

func TestApiChannels_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ChannelIndexWithSpecialCharacters", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up mock data with special characters
		adapter.channelsWithSocketsCount = map[constants.ChannelName]int64{
			"channel-with-dashes":      5,
			"channel_with_underscores": 3,
			"channel.with.dots":        2,
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels", nil)
		c.Request = req

		ChannelIndex(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.ChannelsList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Channels, 3)
	})

	t.Run("ChannelShowWithEmptyInfo", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		adapter.channelSocketsCount = 5

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/public-channel?info=", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "public-channel"}}

		ChannelShow(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "public-channel", response["Name"])
		// No additional fields should be present
		assert.Nil(t, response["user_count"])
		assert.Nil(t, response["subscription_count"])
		assert.Nil(t, response["cache"])
	})

	t.Run("ChannelUsersWithNilMembers", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up nil members (edge case)
		adapter.channelMembers = nil

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("app", app)

		req := httptest.NewRequest("GET", "/channels/presence-channel/users", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}

		ChannelUsers(c, server)

		assert.Equal(t, http.StatusOK, w.Code)

		var response pusherClient.Users
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.List, 0)
	})
}

func TestApiChannels_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("FullWorkflow", func(t *testing.T) {
		server, adapter, _, cancel := createTestServerForApiChannels()
		defer cancel()
		app := createTestAppForApiChannels()

		// Set up comprehensive mock data
		adapter.channelsWithSocketsCount = map[constants.ChannelName]int64{
			"public-channel":   5,
			"private-channel":  3,
			"presence-channel": 2,
		}
		adapter.presenceChannelsWithUsersCount = map[constants.ChannelName]int64{
			"presence-channel": 2,
		}
		adapter.channelMembers = map[constants.SocketID]*pusherClient.MemberData{
			"socket-1": {UserID: "user-1", UserInfo: map[string]string{"name": "User 1"}},
			"socket-2": {UserID: "user-2", UserInfo: map[string]string{"name": "User 2"}},
		}

		// Test ChannelIndex
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Set("app", app)
		req1 := httptest.NewRequest("GET", "/channels", nil)
		c1.Request = req1
		ChannelIndex(c1, server)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Test ChannelShow
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Set("app", app)
		req2 := httptest.NewRequest("GET", "/channels/presence-channel?info=user_count,subscription_count", nil)
		c2.Request = req2
		c2.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}
		ChannelShow(c2, server)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Test ChannelUsers
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Set("app", app)
		req3 := httptest.NewRequest("GET", "/channels/presence-channel/users", nil)
		c3.Request = req3
		c3.Params = gin.Params{{Key: "channel_name", Value: "presence-channel"}}
		ChannelUsers(c3, server)
		assert.Equal(t, http.StatusOK, w3.Code)

		// Test TerminateUserConnections
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Set("app", app)
		req4 := httptest.NewRequest("POST", "/users/user-1/terminate_connections", nil)
		c4.Request = req4
		c4.Params = gin.Params{{Key: "user_id", Value: "user-1"}}
		TerminateUserConnections(c4, server)
		assert.Equal(t, http.StatusOK, w4.Code)
	})
}
