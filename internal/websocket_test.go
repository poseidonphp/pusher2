package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// TEST HELPERS
// ============================================================================

// createTestAppForWebSocket creates a test app with sensible defaults for WebSocket tests
func createTestAppForWebSocket() *apps.App {
	app := &apps.App{
		ID:                           "test-app",
		Key:                          "test-key",
		Secret:                       "test-secret",
		MaxPresenceMembersPerChannel: 100,
		Enabled:                      true,
		EnableClientMessages:         true,
		MaxChannelNameLength:         200,
		MaxEventNameLength:           200,
		MaxEventPayloadInKb:          10,
		ActivityTimeout:              120,
		AuthorizationTimeoutSeconds:  30,
		RequireChannelAuthorization:  false,
	}
	app.SetMissingDefaults()
	return app
}

// createTestServer creates a server instance for testing
func createTestServer(t *testing.T) (*Server, context.CancelFunc) {
	app := createTestAppForWebSocket()
	config := &config.ServerConfig{
		Applications:   []apps.App{*app},
		MetricsEnabled: false,
		QueueDriver:    "local",
	}

	ctx, cancel := context.WithCancel(context.Background())
	server, err := NewServer(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, server)

	return server, cancel
}

// createWebSocketServer creates an HTTP test server that handles WebSocket upgrades
func createWebSocketServer(t *testing.T, server *Server) *httptest.Server {
	app := server.config.Applications[0]
	var clientWebSocket *WebSocket

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle WebSocket upgrade
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}

		// Create WebSocket instance
		socketID := constants.SocketID(fmt.Sprintf("test-socket-%d", time.Now().UnixNano()))
		clientWebSocket = createWebSocket(socketID, conn, server, &app)
		require.NotNil(t, clientWebSocket)

		// Add socket to server
		err = server.Adapter.AddSocket(app.ID, clientWebSocket)
		if err != nil {
			t.Errorf("Failed to add socket to server: %v", err)
			return
		}

		// Start listening for messages
		go clientWebSocket.Listen()
		time.Sleep(time.Millisecond * 100)

		// Send connection established message
		clientWebSocket.Send(payloads.EstablishPack(clientWebSocket.ID, app.ActivityTimeout))
	}))

}

// createWebSocketConnection creates a WebSocket connection to the test server
func createWebSocketConnection(t *testing.T, serverURL string) (net.Conn, *wsutil.Reader, *wsutil.Writer) {
	wsURL := strings.Replace(serverURL, "http://", "ws://", 1) + "/"

	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), wsURL)
	require.NoError(t, err)
	require.NotNil(t, conn)

	reader := wsutil.NewReader(conn, ws.StateClientSide)
	writer := wsutil.NewWriter(conn, ws.StateClientSide, ws.OpText)
	time.Sleep(100 * time.Millisecond)

	return conn, reader, writer
}

// readMessage reads a message from the WebSocket connection
func readMessage(t *testing.T, conn net.Conn) []byte {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	message, _, err := wsutil.ReadServerData(conn)
	require.NoError(t, err)
	if len(message) == 0 {
		t.Fatal("Received empty message")
	}
	return message
}

// sendMessage sends a message over the WebSocket connection
func sendMessage(t *testing.T, writer *wsutil.Writer, message []byte) {
	_, err := writer.Write(message)
	require.NoError(t, err)
	err = writer.Flush()
	require.NoError(t, err)
}

// generateAuthSignature generates a valid auth signature for private/presence channels
func generateAuthSignature(socketID constants.SocketID, channelName, channelData string, app apps.App) string {
	stringParts := []string{string(socketID), channelName}
	if channelData != "" {
		stringParts = append(stringParts, channelData)
	}
	decodedString := strings.Join(stringParts, ":")
	return fmt.Sprintf("%s:%s", app.Key, util.HmacSignature(decodedString, app.Secret))
}

// generateSigninSignature generates a valid signin signature
func generateSigninSignature(socketID constants.SocketID, userData string, appKey string, appSecret string) string {
	decodedString := fmt.Sprintf("%s::user::%s", socketID, userData)
	signedString := util.HmacSignature(decodedString, appSecret)
	return fmt.Sprintf("%s:%s", appKey, signedString)
}

// cleanupTestServer properly cleans up test resources
func cleanupTestServer(t *testing.T, server *Server, httpServer *httptest.Server, cancel context.CancelFunc) {
	if httpServer != nil {
		httpServer.Close()
	}

	httpServer.CloseClientConnections()

	if server != nil {
		server.CloseAllLocalSockets()

		// Shutdown cache and queue managers directly
		if server.CacheManager != nil {
			server.CacheManager.Shutdown()
		}
		if server.QueueManager != nil {
			server.QueueManager.Shutdown(context.Background())
		}

		// Cancel the context to shut down background goroutines
		if cancel != nil {
			cancel()
		}
		// Give more time for cleanup to complete and avoid race conditions
		time.Sleep(200 * time.Millisecond)
		server = nil
		httpServer = nil
	}
}

// ============================================================================
// CONNECTION TESTS
// ============================================================================

func TestWebSocketConnectionEstablishment(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, _ := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message
	message := readMessage(t, conn)

	var payload payloads.Payload
	err := json.Unmarshal(message, &payload)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherConnectionEstablished, payload.Event)

	// Parse the data to get socket ID
	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(payload.Data), &establishData)
	require.NoError(t, err)
	assert.NotEmpty(t, establishData.SocketID)
	assert.Equal(t, 120, establishData.ActivityTimeout)
}

func TestWebSocketPingPong(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)
	fmt.Println("read connection established")

	// Send ping from client
	pingMessage := payloads.PayloadPack(constants.PusherPing, "")
	sendMessage(t, writer, pingMessage)
	fmt.Println("sent ping")

	// Read pong response from server
	message := readMessage(t, conn)
	fmt.Println("read pong")

	var payload payloads.Payload
	err := json.Unmarshal(message, &payload)
	require.NoError(t, err)
	fmt.Println("received pong:", string(message))
	assert.Equal(t, constants.PusherPong, payload.Event)
	fmt.Println("finished pingpong")
}

func TestWebSocketConnectionClosure(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, _ := createWebSocketConnection(t, httpServer.URL)

	t.Cleanup(func() {
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Close the connection
	err := conn.Close()
	assert.NoError(t, err)

	// Try to read from closed connection should fail
	_, _, err = wsutil.ReadServerData(conn)
	assert.Error(t, err)
}

// ============================================================================
// CHANNEL SUBSCRIPTION TESTS
// ============================================================================

func TestPublicChannelSubscription(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message first
	readMessage(t, conn)

	// Subscribe to public channel
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "public-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	message := readMessage(t, conn)

	var response payloads.SubscriptionSucceeded
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, response.Event)
	assert.Equal(t, "public-channel", response.Channel)
}

func TestPublicChannelUnsubscription(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message first
	readMessage(t, conn)

	// Subscribe to public channel first
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "public-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	readMessage(t, conn)

	// Now unsubscribe
	unsubscribePayload := payloads.UnsubscribePayload{
		Event: constants.PusherUnsubscribe,
		Data: payloads.UnsubscribeData{
			Channel: "public-channel",
		},
	}

	unsubscribeMessage, err := json.Marshal(unsubscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, unsubscribeMessage)

	// No response expected for unsubscription, but connection should still be alive
	// Give a moment for unsubscription to complete
	time.Sleep(50 * time.Millisecond)

	// Test with a ping
	pingMessage := payloads.PayloadPack(constants.PusherPing, "")
	sendMessage(t, writer, pingMessage)

	_, _, err = wsutil.ReadServerData(conn)
	assert.NoError(t, err)
}

func TestPrivateChannelSubscription(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

	// Subscribe to private channel with auth
	channelName := "private-channel"
	app := server.config.Applications[0]
	auth := generateAuthSignature(establishData.SocketID, channelName, "", app)

	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: channelName,
			Auth:    auth,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	message = readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, response.Event)
}

func TestPrivateChannelSubscriptionInvalidAuth(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

	// Subscribe to private channel with invalid auth
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "private-channel",
			Auth:    "invalid-auth",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription error response
	message = readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherSubscriptionError, response.Event)
}

func TestPresenceChannelSubscription(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// defer conn.Close()

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

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
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	message = readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, response.Event)

	// Parse the data to verify presence data
	var data map[string]interface{}
	err = json.Unmarshal([]byte(response.Data), &data)
	require.NoError(t, err)
	assert.Contains(t, data, "presence")
}

// ============================================================================
// USER SIGNIN TESTS
// ============================================================================

func TestUserSignin(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})
	server.config.Applications[0].RequireChannelAuthorization = true

	// defer conn.Close()

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

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
	require.NoError(t, err)
	sendMessage(t, writer, signinMessage)

	// Read signin success response
	message = readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherSigninSuccess, response.Event)
}

func TestUserSigninInvalidAuth(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})
	server.config.Applications[0].RequireChannelAuthorization = true

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

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
	require.NoError(t, err)
	sendMessage(t, writer, signinMessage)

	// Connection should be closed due to invalid auth
	var authResponsePayload payloads.Payload
	message = readMessage(t, conn)
	err = json.Unmarshal(message, &authResponsePayload)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, authResponsePayload.Event)

	var errPayload payloads.ErrData
	err = json.Unmarshal([]byte(authResponsePayload.Data), &errPayload)
	require.NoError(t, err)
	assert.Equal(t, util.ErrCodeUnauthorizedConnection, errPayload.Code)
}

func TestSigninTimeout(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, _ := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})
	server.config.Applications[0].RequireChannelAuthorization = true
	server.config.Applications[0].AuthorizationTimeoutSeconds = 1 // Set short timeout for test

	// Read connection established message first
	readMessage(t, conn)

	// Wait for longer than the authorization timeout
	time.Sleep(2 * time.Second)

	// Try to read from connection should fail as it should be closed
	_, _, err := wsutil.ReadServerData(conn)
	assert.Error(t, err)
}

func TestUserIsSubscribedToUserSpecificChannelAfterSignin(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})
	server.config.Applications[0].RequireChannelAuthorization = true

	// Read connection established message first
	message := readMessage(t, conn)

	var establishPayload payloads.Payload
	err := json.Unmarshal(message, &establishPayload)
	require.NoError(t, err)

	var establishData payloads.EstablishData
	err = json.Unmarshal([]byte(establishPayload.Data), &establishData)
	require.NoError(t, err)

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
	require.NoError(t, err)
	sendMessage(t, writer, signinMessage)

	// Read signin success response
	message = readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherSigninSuccess, response.Event)

	// This should trigger a subscription to the user-specific channel
	userChannelName := constants.SocketRushServerToUserPrefix + "user123"

	// Subscribe to the user-specific channel
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: userChannelName,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response

	message = readMessage(t, conn)

	var userChannelResponse payloads.SubscriptionSucceeded
	err = json.Unmarshal(message, &userChannelResponse)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, userChannelResponse.Event)
	assert.Equal(t, userChannelName, userChannelResponse.Channel)
}

// ============================================================================
// CLIENT EVENT TESTS
// ============================================================================

func TestClientEvent(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Subscribe to public channel first
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "public-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	readMessage(t, conn)

	// Send client event
	clientEvent := payloads.ClientChannelEvent{
		Event:   "client-test-event",
		Channel: "public-channel",
		Data:    json.RawMessage(`{"message": "Hello World"}`),
	}

	clientEventMessage, err := json.Marshal(clientEvent)
	require.NoError(t, err)
	sendMessage(t, writer, clientEventMessage)

	// No response expected for client events, but connection should still be alive
	// Test with a ping
	pingMessage := payloads.PayloadPack(constants.PusherPing, "")
	sendMessage(t, writer, pingMessage)

	_, _, err = wsutil.ReadServerData(conn)
	assert.NoError(t, err)
}

func TestClientEventNotSubscribed(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)

	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Send client event without subscribing
	clientEvent := payloads.ClientChannelEvent{
		Event:   "client-test-event",
		Channel: "public-channel",
		Data:    json.RawMessage(`{"message": "Hello World"}`),
	}

	clientEventMessage, err := json.Marshal(clientEvent)
	require.NoError(t, err)
	sendMessage(t, writer, clientEventMessage)

	// Should receive an error response
	message := readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, response.Event)

	var errPayload payloads.ErrData
	err = json.Unmarshal([]byte(response.Data), &errPayload)
	require.NoError(t, err)
	assert.Equal(t, util.ErrCodeClientEventRejected, errPayload.Code)

	// now let's subscribe, but then remove the socket from the SubscribedChannels map
	// simulating something that may have gone wrong, where the client thinks it's in
	// the chanel but the server doesn't
	// Subscribe to a public channel first
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "public-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	readMessage(t, conn)

	// Remove the channel from the SubscribedChannels map
	app := server.config.Applications[0]
	clientSockets := server.Adapter.GetSockets(app.ID, true)
	for clientSocketId, _ := range clientSockets {
		err = server.Adapter.RemoveSocket(app.ID, clientSocketId)
		assert.NoError(t, err)
	}

	// Send client event again
	sendMessage(t, writer, clientEventMessage)

	// Should receive an error response
	message = readMessage(t, conn)

	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, response.Event)

	err = json.Unmarshal([]byte(response.Data), &errPayload)
	require.NoError(t, err)
	assert.Equal(t, util.ErrCodeNotSubscribed, errPayload.Code)

}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

func TestInvalidMessage(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Send invalid JSON
	sendMessage(t, writer, []byte("invalid json"))

	// Should receive an error response
	message := readMessage(t, conn)

	var response payloads.ChannelEvent
	err := json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, response.Event)
}

func TestMessageTooLarge(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Send oversized message
	oversizedMessage := make([]byte, constants.MaxMessageSize+1)
	for i := range oversizedMessage {
		oversizedMessage[i] = 'a'
	}
	sendMessage(t, writer, oversizedMessage)

	// Should receive an error response or connection should be closed

	_, code, err := wsutil.ReadServerData(conn)
	assert.Equal(t, ws.OpCode(0), code) // Connection likely closed
	assert.Contains(t, err.Error(), "timeout")
}

func TestUnsupportedEvent(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Send unsupported event
	unsupportedEvent := payloads.PayloadPack("unsupported-event", "")
	sendMessage(t, writer, unsupportedEvent)

	// Should receive an error response
	message := readMessage(t, conn)

	var response payloads.Payload
	err := json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, response.Event)
}

func TestInvalidChannelName(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Subscribe to invalid channel name (too long)
	longChannelName := strings.Repeat("a", 300) // Exceeds MaxChannelNameLength
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: longChannelName,
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Should receive an error response
	message := readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherError, response.Event)
}

// ============================================================================
// CACHE CHANNEL TESTS
// ============================================================================

func TestCacheChannel(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	conn, _, writer := createWebSocketConnection(t, httpServer.URL)
	t.Cleanup(func() {
		conn.Close()
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Read connection established message first
	readMessage(t, conn)

	// Subscribe to cache channel
	subscribePayload := payloads.SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: payloads.SubscribeChannelData{
			Channel: "cache-channel",
		},
	}

	subscribeMessage, err := json.Marshal(subscribePayload)
	require.NoError(t, err)
	sendMessage(t, writer, subscribeMessage)

	// Read subscription success response
	message := readMessage(t, conn)

	var response payloads.Payload
	err = json.Unmarshal(message, &response)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, response.Event)

	// Should receive cache miss event
	message = readMessage(t, conn)

	var cacheMissResponse payloads.Payload
	err = json.Unmarshal(message, &cacheMissResponse)
	require.NoError(t, err)
	assert.Equal(t, constants.PusherCacheMiss, cacheMissResponse.Event)
}

// ============================================================================
// CONCURRENT CONNECTION TESTS
// ============================================================================

func TestConcurrentConnections(t *testing.T) {
	server, cancel := createTestServer(t)
	httpServer := createWebSocketServer(t, server)
	defer cleanupTestServer(t, server, httpServer, cancel)

	t.Cleanup(func() {
		cleanupTestServer(t, server, httpServer, cancel)
	})

	// Create multiple connections
	numConnections := 3
	connections := make([]net.Conn, numConnections)
	writers := make([]*wsutil.Writer, numConnections)

	for i := 0; i < numConnections; i++ {
		conn, _, writer := createWebSocketConnection(t, httpServer.URL)
		connections[i] = conn
		writers[i] = writer
		defer conn.Close()

		// Read connection established message
		readMessage(t, conn)
	}

	// All connections should be alive
	for i := 0; i < numConnections; i++ {
		pingMessage := payloads.PayloadPack(constants.PusherPing, "")
		sendMessage(t, writers[i], pingMessage)

		_, _, err := wsutil.ReadServerData(connections[i])
		assert.NoError(t, err, "Connection %d should be alive", i)
	}
}
