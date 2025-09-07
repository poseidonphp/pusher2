package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pusher/internal/apps"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"

	"github.com/gorilla/websocket"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a test app
func createTestAppForWebSocket() *apps.App {
	app := &apps.App{
		ID:                           "test-app",
		Key:                          "test-key",
		Secret:                       "test-secret",
		MaxPresenceMembersPerChannel: 0,
		Enabled:                      true,
		EnableClientMessages:         true,
		MaxChannelNameLength:         200,
		MaxEventNameLength:           200,
		MaxEventPayloadInKb:          10,
		ReadTimeout:                  30 * time.Second,
		AuthorizationTimeout:         30 * time.Second,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test server with WebSocket support
func createTestServerWithWebSocket(t *testing.T) (*Server, *httptest.Server) {
	// Create a test app
	app := createTestAppForWebSocket()

	// Create server config
	config := &config.ServerConfig{
		Applications: []apps.App{*app},
	}

	// Create server
	server, err := NewServer(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, server)

	// Create HTTP test server
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle WebSocket upgrade
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}

		// Create WebSocket instance
		ws := &WebSocket{
			ID:                 constants.SocketID("test-socket-" + fmt.Sprintf("%d", time.Now().UnixNano())),
			app:                app,
			SubscribedChannels: make(map[constants.ChannelName]*Channel),
			PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
			conn:               conn,
			closed:             false,
			server:             server,
		}

		// Add socket to server
		addErr := server.Adapter.AddSocket(app.ID, ws)
		if addErr != nil {
			t.Errorf("Failed to add socket to server: %v", addErr)
			return
		}

		// Start listening
		go ws.Listen()

		// Send connection established message
		ws.Send(payloads.EstablishPack(ws.ID, 30))
	}))

	return server, httpServer
}

// Helper function to create a WebSocket connection
func createWebSocketConnection(t *testing.T, serverURL string) *websocket.Conn {
	// Convert http:// to ws://
	wsURL := strings.Replace(serverURL, "http://", "ws://", 1) + "/"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	return conn
}

// Helper function to generate a valid auth signature for private/presence channels
// func generateAuthSignature(socketID constants.SocketID, channelName, channelData, appSecret string) string {
func generateAuthSignature(socketID constants.SocketID, channelName, channelData string, app apps.App) string {
	stringParts := []string{string(socketID), channelName}
	if channelData != "" {
		stringParts = append(stringParts, channelData)
	}
	decodedString := strings.Join(stringParts, ":")
	return fmt.Sprintf("%s:%s", app.Key, util.HmacSignature(decodedString, app.Secret))
}

// Helper function to generate a valid signin signature
func generateSigninSignature(socketID constants.SocketID, userData string, appKey string, appSecret string) string {
	decodedString := fmt.Sprintf("%s::user::%s", socketID, userData)
	signedString := util.HmacSignature(decodedString, appSecret)
	return fmt.Sprintf("%s:%s", appKey, signedString)
}

// ============================================================================
// CONNECTION ESTABLISHMENT TESTS
// ============================================================================

func TestWebSocketConnectionEstablishment(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	// Create WebSocket connection
	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherConnectionEstablished, payload.Event)

	// Parse the data to get socket ID
	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(payload.Data), &establishData)
	assert.NoError(t, err)
	assert.NotEmpty(t, establishData.SocketID)
	assert.Equal(t, 30, establishData.ActivityTimeout)
}

// ============================================================================
// PING/PONG TESTS
// ============================================================================

func TestWebSocketPingPong(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Send ping from client
	pingMessage := payloads.PayloadPack(constants.PusherPing, "")
	err = conn.WriteMessage(websocket.TextMessage, pingMessage)
	assert.NoError(t, err)

	// Read pong response from server
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherPong, payload.Event)
}

// ============================================================================
// PUBLIC CHANNEL SUBSCRIPTION TESTS
// ============================================================================

func TestWebSocketConnectionTimeout(t *testing.T) {
	// TODO: Implement test for connection timeout if require_channel_authorization is enabled
	//  as well as the server configuration for connection timeout
}

func TestPublicSubscription(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()
	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	t.Run("Subscribe", func(t *testing.T) {
		// Subscribe to public channel
		subscribePayload := payloads.SubscribePayload{
			Event: constants.PusherSubscribe,
			Data: payloads.SubscribeChannelData{
				Channel: "public-channel",
			},
		}

		subscribeMessage, err := json.Marshal(subscribePayload)
		assert.NoError(t, err)

		err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
		assert.NoError(t, err)

		// Read subscription success response
		_, message, err := conn.ReadMessage()
		assert.NoError(t, err)

		var payload payloads.SubscriptionSucceeded
		err = json.Unmarshal(message, &payload)
		assert.NoError(t, err)
		assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, payload.Event)
		assert.Equal(t, "public-channel", payload.Channel)

	})

	t.Run("Unsubscribe", func(t *testing.T) {
		// Now unsubscribe
		unsubscribePayload := payloads.UnsubscribePayload{
			Event: constants.PusherUnsubscribe,
			Data: payloads.UnsubscribeData{
				Channel: "public-channel",
			},
		}

		unsubscribeMessage, err := json.Marshal(unsubscribePayload)
		assert.NoError(t, err)

		err = conn.WriteMessage(websocket.TextMessage, unsubscribeMessage)
		assert.NoError(t, err)

		// No response expected for unsubscription
		// The connection should still be alive
		err = conn.WriteMessage(websocket.TextMessage, payloads.PayloadPack(constants.PusherPing, ""))
		assert.NoError(t, err)

		_, _, err = conn.ReadMessage()
		assert.NoError(t, err)
	})
}

// ============================================================================
// PRIVATE CHANNEL SUBSCRIPTION TESTS
// ============================================================================

func TestWebSocketPrivateChannelSubscription(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var establishPayload payloads.Payload
	err = json.Unmarshal(message, &establishPayload)
	assert.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	assert.NoError(t, err)

	// Subscribe to private channel with auth
	channelData := ``
	app := server.config.Applications[0]
	channelName := "private-channel"
	auth := generateAuthSignature(establishData.SocketID, channelName, channelData, app)

	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel:     channelName,
			Auth:        auth,
			ChannelData: channelData,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Read subscription success response
	_, message, err = conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, payload.Event)
}

func TestWebSocketPrivateChannelSubscriptionInvalidAuth(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var establishPayload payloads.Payload
	err = json.Unmarshal(message, &establishPayload)
	assert.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	assert.NoError(t, err)

	// Subscribe to private channel with invalid auth
	channelData := `{"user_id": "user123"}`
	invalidAuth := "invalid-auth"

	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel:     "private-channel",
			Auth:        invalidAuth,
			ChannelData: channelData,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Read subscription error response
	_, message, err = conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherSubscriptionError, payload.Event)
}

// ============================================================================
// PRESENCE CHANNEL SUBSCRIPTION TESTS
// ============================================================================

func TestWebSocketPresenceChannelSubscription(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var establishPayload payloads.Payload
	err = json.Unmarshal(message, &establishPayload)
	assert.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	assert.NoError(t, err)

	// Subscribe to presence channel with auth
	channelData := `{"user_id": "user123", "user_info": {"name": "Test User"}}`
	channelName := "presence-channel"
	app := server.config.Applications[0]
	auth := generateAuthSignature(establishData.SocketID, channelName, channelData, app)

	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel:     channelName,
			Auth:        auth,
			ChannelData: channelData,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Read subscription success response
	_, message, err = conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, payload.Event)

	// Parse the data to verify presence data
	var data map[string]interface{}
	err = json.Unmarshal([]byte(payload.Data), &data)
	assert.NoError(t, err)
	assert.Contains(t, data, "presence")
}

func TestPresenceMemberLimitExceeded(t *testing.T) {
	// todo: Implement test for presence member limit exceeded
}

func TestPresenceMemberSizeLimitExceeded(t *testing.T) {
	// todo: Implement test for presence member size limit exceeded
}

// ============================================================================
// USER SIGNIN TESTS
// ============================================================================

func TestWebSocketUserSignin(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var establishPayload payloads.Payload
	err = json.Unmarshal(message, &establishPayload)
	assert.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	assert.NoError(t, err)

	// Sign in user
	userData := `{"id": "user123", "user_info": {"name": "Test User"}}`
	auth := generateSigninSignature(establishData.SocketID, userData, "test-key", "test-secret")

	signinPayload := UserSigninPayload{
		Event: constants.PusherSignin,
		Data: UserSigninData{
			UserData: userData,
			Auth:     auth,
		},
	}

	signinMessage, err := json.Marshal(signinPayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, signinMessage)
	assert.NoError(t, err)

	// Read signin success response
	_, message, err = conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherSigninSuccess, payload.Event)
}

func TestWebSocketUserSigninInvalidAuth(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var establishPayload payloads.Payload
	err = json.Unmarshal(message, &establishPayload)
	assert.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	assert.NoError(t, err)

	// Sign in user with invalid auth
	userData := `{"id": "user123", "user_info": {"name": "Test User"}}`
	invalidAuth := "invalid-auth"

	signinPayload := UserSigninPayload{
		Event: constants.PusherSignin,
		Data: UserSigninData{
			UserData: userData,
			Auth:     invalidAuth,
		},
	}

	signinMessage, err := json.Marshal(signinPayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, signinMessage)
	assert.NoError(t, err)

	// Connection should be closed due to invalid auth
	_, _, _ = conn.ReadMessage() // first message is the notice about closing the connection

	_, _, err = conn.ReadMessage() // second message is the actual error

	assert.Error(t, err)
}

// ============================================================================
// CLIENT EVENT TESTS
// ============================================================================

func TestWebSocketClientEvent(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Subscribe to public channel first
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "public-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Read subscription success response
	_, _, err = conn.ReadMessage()
	assert.NoError(t, err)

	// Send client event
	clientEvent := payloads.ClientChannelEvent{
		Event:   "client-test-event",
		Channel: "public-channel",
		Data:    json.RawMessage(`{"message": "Hello World"}`),
	}

	clientEventMessage, err := json.Marshal(clientEvent)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, clientEventMessage)
	assert.NoError(t, err)

	// No response expected for client events (they are broadcast to other subscribers)
	// The connection should still be alive
	err = conn.WriteMessage(websocket.TextMessage, payloads.PayloadPack(constants.PusherPing, ""))
	assert.NoError(t, err)

	_, _, err = conn.ReadMessage()
	assert.NoError(t, err)
}

func TestWebSocketClientEventNotSubscribed(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Send client event without subscribing
	clientEvent := payloads.ClientChannelEvent{
		Event:   "client-test-event",
		Channel: "public-channel",
		Data:    json.RawMessage(`{"message": "Hello World"}`),
	}

	clientEventMessage, err := json.Marshal(clientEvent)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, clientEventMessage)
	assert.NoError(t, err)

	// Should receive an error response
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, payload.Event)
}

// ============================================================================
// CONNECTION CLOSURE TESTS
// ============================================================================

func TestWebSocketConnectionClosure(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Close the connection
	err = conn.Close()
	assert.NoError(t, err)

	// Try to read from closed connection
	_, _, err = conn.ReadMessage()
	assert.Error(t, err)
}

// ============================================================================
// CONCURRENT CONNECTION TESTS
// ============================================================================

func TestWebSocketConcurrentConnections(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	// Create multiple connections
	numConnections := 5
	connections := make([]*websocket.Conn, numConnections)

	for i := 0; i < numConnections; i++ {
		conn := createWebSocketConnection(t, httpServer.URL)
		connections[i] = conn
		defer conn.Close()

		// Read connection established message
		_, _, err := conn.ReadMessage()
		assert.NoError(t, err)
	}

	// All connections should be alive
	for i, conn := range connections {
		err := conn.WriteMessage(websocket.TextMessage, payloads.PayloadPack(constants.PusherPing, ""))
		assert.NoError(t, err, "Connection %d should be alive", i)

		_, _, err = conn.ReadMessage()
		assert.NoError(t, err, "Connection %d should be alive", i)
	}
}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

func TestWebSocketInvalidMessage(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Send invalid JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
	assert.NoError(t, err)

	// Should receive an error response
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.ChannelEvent
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, payload.Event)
}

func TestWebSocketUnsupportedEvent(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Send unsupported event
	unsupportedEvent := payloads.PayloadPack("unsupported-event", "")
	err = conn.WriteMessage(websocket.TextMessage, unsupportedEvent)
	assert.NoError(t, err)

	// Should receive an error response
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, payload.Event)
}

// ============================================================================
// CHANNEL VALIDATION TESTS
// ============================================================================

func TestWebSocketInvalidChannelName(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Subscribe to invalid channel name (too long)
	longChannelName := strings.Repeat("a", 300) // Exceeds MaxChannelNameLength
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: longChannelName,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Should receive an error response
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, payload.Event)
}

// ============================================================================
// CACHE CHANNEL TESTS
// ============================================================================

func TestWebSocketCacheChannel(t *testing.T) {
	server, httpServer := createTestServerWithWebSocket(t)
	defer httpServer.Close()
	defer server.CloseAllLocalSockets()

	conn := createWebSocketConnection(t, httpServer.URL)
	defer conn.Close()

	// Read connection established message first
	_, _, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Subscribe to cache channel
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "cache-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	assert.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, subscribeMessage)
	assert.NoError(t, err)

	// Read subscription success response
	_, message, err := conn.ReadMessage()
	assert.NoError(t, err)

	var payload payloads.Payload
	err = json.Unmarshal(message, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, payload.Event)

	// Should receive cache miss event
	_, message, err = conn.ReadMessage()
	assert.NoError(t, err)

	var cacheMissPayload payloads.Payload
	err = json.Unmarshal(message, &cacheMissPayload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherCacheMiss, cacheMissPayload.Event)
}
