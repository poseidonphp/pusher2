package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"pusher/internal/apps"
	"pusher/internal/cache"

	"github.com/gorilla/websocket"
	pusherClient "github.com/pusher/pusher-http-go/v5"

	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

type WebSocket struct {
	ID                 constants.SocketID
	app                *apps.App
	SubscribedChannels map[constants.ChannelName]*Channel
	PresenceData       map[constants.ChannelName]*pusherClient.MemberData
	conn               *websocket.Conn
	closed             bool
	mutex              sync.Mutex
	sendMutex          sync.Mutex
	server             *Server
	userID             string
	userAuthChannel    chan bool
	userHasAuthorized  bool
}

type UserSigninPayload struct {
	Event string         `json:"event"`
	Data  UserSigninData `json:"data"`
}

type UserSigninData struct {
	UserData string `json:"user_data"`
	Auth     string `json:"auth"`
}

type AuthDataUserData struct {
	ID       string `json:"id"`
	UserInfo struct {
		Name string `json:"name"`
	} `json:"user_info"`
	Watchlist []string `json:"watchlist"`
}

// ============================================================================
// PUBLIC METHODS
// ============================================================================

// Send transmits a message to the connected client
// It handles write timeouts and connection closure on failure
func (w *WebSocket) Send(msg []byte) {
	if w.closed || w.conn == nil {
		log.Logger().Debug("connection is closed or nil")
		return
	}

	// Send a message to the client
	w.sendMutex.Lock() // obtain a lock to prevent concurrent websocket writes which would panic
	_ = w.conn.SetWriteDeadline(time.Now().Add(constants.WriteWait))
	if err := w.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		w.closed = true
	}
	w.sendMutex.Unlock()
	// Mark WebSocket message sent in metrics
	if w.server.MetricsManager != nil {
		w.server.MetricsManager.MarkWsMessageSent(w.app.ID, msg)
	}

	// reset the read timeout after a successful send, because the pusher client will reset it on receive
	w.resetReadTimeout()
}

// Close cleanly shuts down the WebSocket connection due to server shutdown
func (w *WebSocket) Close() {
	w.closeConnection(util.ErrCodeServerShuttingDown)
}

// updateChannelMetrics updates channel-related metrics
func (w *WebSocket) updateChannelMetrics() {
	// Get all channels and categorize them
	if w.server.MetricsManager == nil {
		return
	}

	// channels := w.server.Adapter.GetChannelsWithSocketsCount(w.app.ID, false)
	//
	// // Calculate channel counts by type
	// totalChannelsCount := int64(len(channels))
	// presenceChannelsCount := int64(0)
	// privateChannelsCount := int64(0)
	// publicChannelsCount := int64(0)
	//
	// for channelName := range channels {
	// 	if util.IsPresenceChannel(channelName) {
	// 		presenceChannelsCount++
	// 	} else if util.IsPrivateChannel(channelName) {
	// 		privateChannelsCount++
	// 	} else {
	// 		publicChannelsCount++
	// 	}
	// }
	//
	// // Update metrics
	// w.server.MetricsManager.SetChannelsTotal(float64(totalChannelsCount))
	// w.server.MetricsManager.SetPresenceChannels(float64(presenceChannelsCount))
	// w.server.MetricsManager.SetPrivateChannels(float64(privateChannelsCount))
	// w.server.MetricsManager.SetPublicChannels(float64(publicChannelsCount))
}

// Listen continuously reads messages from the WebSocket connection
// It handles incoming messages and responds to client requests
func (w *WebSocket) Listen() {
	defer func() {
		w.tracef("closing listener")
	}()

	w.conn.SetReadLimit(constants.MaxMessageSize)
	w.tracef("starting reader")

	for {
		if w.closed || w.conn == nil {
			w.tracef("connection is closed or nil")
			return
		}

		_, msgRaw, err := w.conn.ReadMessage()
		if err != nil {
			w.handleReadMessageError(err)
			return
		}

		// reset readTimeout
		w.resetReadTimeout()

		// Mark WebSocket message received in metrics
		if w.server.MetricsManager != nil {
			w.server.MetricsManager.MarkWsMessageReceived(w.app.ID, msgRaw)
		}

		// parse incoming payload using msgRaw
		parseErr := w.parseIncomingPayload(msgRaw)
		if parseErr != nil {
			log.Logger().Error("failed to parse incoming payload: ", parseErr)
			w.Send(payloads.ErrorPack(parseErr.Code, parseErr.Message))
		}
	}
}

// WatchForAuthentication sets up a go channel for channel authorization
// When the user is authenticated, the channel is closed
// If the time runs out before the channel is closed, the connection is closed
func (w *WebSocket) WatchForAuthentication() {
	w.tracef("Setting up channel authorization Channel")

	w.mutex.Lock()
	if w.userAuthChannel != nil {
		w.mutex.Unlock()
		w.errorf("Channel authorization Channel already exists")
		return
	}

	w.userAuthChannel = make(chan bool, 1)
	w.mutex.Unlock()

	// set a timeout for the user auth Channel. Listen for the timeout;
	// if the userAuthChannel is closed before the timeout, close the Channel and stop the time
	// if the timer expires, close the websocket
	go func() {
		timer := time.NewTimer(w.app.AuthorizationTimeout)
		defer timer.Stop()

		select {
		case authResult, ok := <-w.userAuthChannel:
			if ok {
				w.mutex.Lock()
				w.userHasAuthorized = authResult
				w.mutex.Unlock()
				w.tracef("User authorization completed: %v", authResult)
			}
		case <-timer.C:
			w.errorf("Channel authorization timeout")
			w.closeConnection(util.ErrCodeUnauthorizedConnection)
			return
		}

		// clean up the channel under lock
		w.mutex.Lock()
		if w.userAuthChannel != nil {
			close(w.userAuthChannel)
			w.userAuthChannel = nil
		}
		w.mutex.Unlock()
	}()
}

// Cleanup performs cleanup operations when the WebSocket is closed
func (w *WebSocket) Cleanup() {
	// Close any remaining channels
	if w.userAuthChannel != nil {
		close(w.userAuthChannel)
		w.userAuthChannel = nil
	}

	// Clear any large data structures
	w.PresenceData = nil
	w.SubscribedChannels = nil

	// Any other cleanup needed
	w.ID = ""
	w.app = nil
	w.conn = nil
	w.closed = false
	w.server = nil
	w.userID = ""
	w.userHasAuthorized = false
}

// ============================================================================
// PRIVATE METHODS
// ============================================================================

// closeConnection closes the WebSocket connection with a specific error code
// It unsubscribes from all channels and removes socket from adapter
func (w *WebSocket) closeConnection(code util.ErrorCode) {
	w.tracef("closing connection with code: %d", code)
	w.unsubscribeFromAllChannels()
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.app != nil && w.server != nil && w.server.Adapter != nil {

		_ = w.server.Adapter.RemoveSocket(w.app.ID, w.ID)
		// Mark disconnection in metrics
		if w.server.MetricsManager != nil {
			w.server.MetricsManager.MarkDisconnection(w.app.ID)
		}
	}

	if w.userID != "" {
		_ = w.server.Adapter.RemoveUser(w.app.ID, w)
	}

	if w.conn != nil {
		w.tracef("sending connection close message to client")
		w.Send(payloads.ErrorPack(code, "closing connection"))
		time.Sleep(600 * time.Millisecond)
		err := w.conn.Close()
		if err != nil {
			w.errorf("Error closing connection: %s", err)
		} else {
			time.Sleep(600 * time.Millisecond)
			w.conn = nil
		}

	}

	if w.closed {
		return
	}
	w.closed = true
	w.Cleanup()
}

// errorf logs a message at the error level, including the socket ID for context.
// This provides consistent error logging with socket identification for debugging.
func (w *WebSocket) errorf(format string, args ...interface{}) {
	// log error
	msg := fmt.Sprintf(format, args...)
	log.Logger().Errorf("[%s] %s", w.ID, msg)
}

// debugf logs a message at the debug level, including the socket ID for context.
// This provides consistent error logging with socket identification for debugging.
func (w *WebSocket) debugf(format string, args ...interface{}) {
	// log error
	msg := fmt.Sprintf(format, args...)
	log.Logger().Debugf("[%s] %s", w.ID, msg)
}

// getPresenceDataForChannel retrieves presence data for a specific Channel
// Returns nil if no presence data exists for the Channel
func (w *WebSocket) getPresenceDataForChannel(channel constants.ChannelName) *pusherClient.MemberData {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if _, ok := w.PresenceData[channel]; ok {
		return w.PresenceData[channel]
	}
	return nil
}

// handleClientEvent processes client-generated events sent to channels.
// It validates that the client is subscribed to the channel, checks app permissions,
// validates event name length and payload size, then broadcasts the event to other
// subscribers in the same channel. For presence channels, it includes the user ID
// in the broadcast.
func (w *WebSocket) handleClientEvent(msgRaw []byte) *util.Error {
	w.tracef("Handling client event")
	var clientChannelEvent payloads.ClientChannelEvent
	err := json.Unmarshal(msgRaw, &clientChannelEvent)
	if err != nil {
		return util.NewError(util.ErrCodeInvalidPayload, err.Error())
	}

	channel := clientChannelEvent.Channel

	w.mutex.Lock()
	if _, ok := w.SubscribedChannels[channel]; !ok {
		w.mutex.Unlock()
		w.errorf("Channel not subscribed: %s", channel)
		w.sendChannelError(channel, util.ErrCodeClientEventRejected, fmt.Sprintf("Client is not subscribed to %s", channel))
		return nil
	}
	w.mutex.Unlock()

	// Validate the client event
	if validationErr := w.validateClientEvent(channel, clientChannelEvent.Event, msgRaw); validationErr != nil {
		return validationErr
	}

	// check to make sure the client is in the channel. Because this is a client event, we can force a onlyLocal check
	if !w.server.Adapter.IsInChannel(w.app.ID, channel, w.ID, true) {
		w.tracef("Socket %s is not in Channel %s", w.ID, channel)
		return nil
	}

	// TODO: Add rate limiting here to prevent clients from sending too many events
	// Check if the client has exceeded the rate limit for this channel/app
	// If rate limited, send an error response and return early
	// Consider using a sliding window or token bucket algorithm
	// Rate limits should be configurable per app and potentially per channel

	channelObj := w.SubscribedChannels[channel]
	var userId string
	if channelObj.Type == constants.ChannelTypePresence {
		if member, ok := w.PresenceData[channel]; ok {
			userId = member.UserID
		} else {
			w.errorf("Member not found in presence data")
			return nil
		}
	}

	message := payloads.ChannelEvent{
		Event:   clientChannelEvent.Event,
		Channel: channel,
		Data:    clientChannelEvent.Data,
		UserID:  userId,
	}
	_messageData := message.ToJson(true)

	_ = w.server.Adapter.Send(w.app.ID, channel, _messageData, w.ID)

	w.server.QueueManager.SendClientEvent(w.app, channel, clientChannelEvent.Event, string(clientChannelEvent.Data), w.ID, userId)

	return nil
}

// handleMemberAddedEvent handles the member_added event logic for presence channels.
// It checks if the member already exists in the channel to avoid duplicate events,
// sends a webhook notification, broadcasts the member_added event to other subscribers,
// and updates the local members map. This ensures proper presence channel state management.
func (w *WebSocket) handleMemberAddedEvent(channelObj *Channel, joinResponse *ChannelJoinResponse, members map[string]*pusherClient.MemberData) {
	w.mutex.Lock()
	memberExists := false
	if _, exists := members[joinResponse.Member.UserID]; exists {
		memberExists = true
	}
	w.mutex.Unlock()

	if memberExists {
		w.tracef("Member already exists, not sending member added event")
		return
	}

	w.tracef("Member does not exist, sending member added event")
	// send webhook for member added
	w.server.QueueManager.SendMemberAdded(w.app, channelObj.Name, joinResponse.Member.UserID)

	// send member_added event to other nodes and sockets
	memberAdded := payloads.ChannelEvent{
		Event:   constants.PusherInternalPresenceMemberAdded,
		Channel: channelObj.Name,
		Data:    &joinResponse.Member,
	}
	_memberAdded := memberAdded.ToJson(true)
	_ = w.server.Adapter.Send(w.app.ID, channelObj.Name, _memberAdded, w.ID)

	w.mutex.Lock()
	if members == nil {
		members = make(map[string]*pusherClient.MemberData)
	}
	members[joinResponse.Member.UserID] = joinResponse.Member
	w.mutex.Unlock()
}

// handlePresenceChannelSubscription handles the complex logic for presence channel subscriptions.
// It validates member data, retrieves existing channel members, stores the member's presence data,
// handles member_added events if needed, sends the subscription success response with presence
// data, and handles cache operations. This method orchestrates the complete presence channel
// subscription flow including member management and event broadcasting.
func (w *WebSocket) handlePresenceChannelSubscription(channelObj *Channel, joinResponse *ChannelJoinResponse) *util.Error {
	// Continue with presence data
	if joinResponse.Member.UserID == "" {
		w.errorf("Member data is empty")
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	members := w.server.Adapter.GetChannelMembers(w.app.ID, channelObj.Name, false)
	log.Logger().Debugf("Members: %v", members)

	w.mutex.Lock()
	w.PresenceData[channelObj.Name] = joinResponse.Member
	w.mutex.Unlock()

	log.Logger().Debugf("PresenceData: %v", w.PresenceData)

	// If the member already exists in the Channel, don't resend the member_added event
	w.tracef("Checking if %s exists in Channel %s", joinResponse.Member.UserID, channelObj.Name)
	w.tracef("Members: %v", members)

	w.handleMemberAddedEvent(channelObj, joinResponse, members)
	w.sendPresenceSubscriptionSuccess(channelObj, members)
	w.sendCacheIfNeeded(channelObj)

	return nil
}

// handleReadMessageError processes WebSocket read errors
// Differentiates between expected and unexpected closure events
func (w *WebSocket) handleReadMessageError(err error) {
	expectedCodes := []int{websocket.CloseGoingAway, websocket.CloseNormalClosure}
	if websocket.IsUnexpectedCloseError(err, expectedCodes...) {
		var e *websocket.CloseError
		if errors.As(err, &e) {
			w.debugf("Unexpected close error: %s", e.Error())
		} else {
			w.debugf("Unexpected close error: %s", err)
		}
		// Mark error in metrics
		if w.server.MetricsManager != nil {
			w.server.MetricsManager.MarkError("websocket_abnormal_closure", w.app.ID)
		}
		w.closeConnection(util.ErrCodeWebsocketAbnormalClosure)
	} else {
		w.tracef("Closing connection as expected: %s", err.Error())
		// Mark error in metrics
		if w.server.MetricsManager != nil {
			w.server.MetricsManager.MarkError("websocket_expected_closure", w.app.ID)
		}
		w.closeConnection(util.ErrCodeCloseExpected)
	}
}

// handleSubscribeRequest processes a client's channel subscription request.
// It validates the channel name, handles special user channels, performs channel authorization
// if required, joins the channel through the adapter, manages subscription state, handles
// presence channel logic, and sends appropriate success/error responses. This is the main
// entry point for all channel subscription logic.
func (w *WebSocket) handleSubscribeRequest(msgRaw []byte) *util.Error {
	w.tracef("Handling subscribe request")
	if w.server.Closing {
		return util.NewError(util.ErrCodeGenericReconnect)
	}

	var subscribePayload payloads.SubscribePayload
	err := json.Unmarshal(msgRaw, &subscribePayload)
	if err != nil {
		w.errorf("Error unmarshalling subscribe payload: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	channel := subscribePayload.Data.Channel

	// Special handling for user-specific channels (e.g., "server-to-user-123")
	// These are automatically subscribed to when a user signs in
	if w.userID != "" && channel == constants.SocketRushServerToUserPrefix+w.userID {
		w.subscribeToUserChannel()
		return nil
	}

	// Validate channel name format and length against app limits
	channelErr := util.ValidateChannelName(channel, w.app.MaxChannelNameLength)
	if channelErr != nil {
		w.errorf("%s", channelErr)
		w.Send(payloads.ErrorPack(util.ErrCodeUnauthorizedConnection, channelErr.Error()))
		return util.NewError(util.ErrCodeInvalidChannel)
	}

	// Create channel object and determine its type based on naming conventions
	channelObj := CreateChannelFromString(w.app, channel)

	// Attempt to join the channel (handles auth validation, member limits, etc.)
	joinResponse := channelObj.Join(w.server.Adapter, w, subscribePayload)
	if !joinResponse.Success {
		w.sendSubscriptionError(channelObj.Name, joinResponse.Type, joinResponse.Message, joinResponse.ErrorCode)
		switch joinResponse.ErrorCode {
		case util.ErrCodeSubscriptionAccessDenied:
			w.debugf("Subscription access denied: %s", joinResponse.Message)
			w.sendSubscriptionError(channelObj.Name, "AuthError", joinResponse.Message, joinResponse.ErrorCode)
			return util.NewError(util.ErrCodeSubscriptionAccessDenied)
		default:
			w.errorf("Failed to join Channel: %s", joinResponse.Message)
			w.sendSubscriptionError(channelObj.Name, "Unknown", joinResponse.Message, joinResponse.ErrorCode)
			return util.NewError(util.ErrCodeGenericReconnect)
		}
	}

	// Track the channel in our local subscription map to avoid duplicate subscriptions
	w.mutex.Lock()
	if _, ok := w.SubscribedChannels[channel]; !ok {
		w.SubscribedChannels[channelObj.Name] = channelObj
	}
	w.mutex.Unlock()

	// Handle user authentication flow for private/presence channels
	w.mutex.Lock()
	needsAuth := channelObj.RequiresAuth && !w.userHasAuthorized && w.userAuthChannel != nil
	if needsAuth {
		// Extract user ID from presence channel member data if not already set
		// This allows presence channels to establish user identity during subscription
		if w.userID == "" && joinResponse.Member != nil && joinResponse.Member.UserID != "" {
			w.userID = joinResponse.Member.UserID
		}
	}
	w.mutex.Unlock()

	if needsAuth {
		w.signalAuthenticated()
	}

	// Notify when this is the first connection to a channel (useful for webhooks)
	if joinResponse.ChannelConnections == 1 {
		w.server.QueueManager.SendChannelOccupied(w.app, channelObj.Name)
		log.Logger().Debug("Channel occupied")
	}

	// Simple success response for non-presence channels
	if channelObj.Type != constants.ChannelTypePresence {
		w.Send(payloads.SubscriptionSucceededPack(channelObj.Name, "{}"))
		w.sendCacheIfNeeded(channelObj)
		return nil
	}

	// Presence channels require additional member management and event handling
	return w.handlePresenceChannelSubscription(channelObj, joinResponse)
}

// handleUnsubscribeRequest processes a client's channel unsubscription request.
// It validates the request payload, checks if the client is subscribed to the channel,
// and delegates to unsubscribeFromChannel for the actual unsubscription logic.
// Returns an error if the request is invalid or the channel is not subscribed.
func (w *WebSocket) handleUnsubscribeRequest(msgRaw []byte) *util.Error {
	w.tracef("Handling unsubscribe request")

	var unsubscribePayload payloads.UnsubscribePayload
	err := json.Unmarshal(msgRaw, &unsubscribePayload)
	if err != nil {
		w.errorf("Error unmarshalling subscribe payload: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	var channelObj *Channel

	w.mutex.Lock()
	if ch, ok := w.SubscribedChannels[unsubscribePayload.Data.Channel]; ok {
		channelObj = ch
	}
	w.mutex.Unlock()
	if channelObj == nil {
		w.errorf("Channel not subscribed: %s", unsubscribePayload.Data.Channel)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	w.unsubscribeFromChannel(channelObj)

	return nil
}

// handleUserSignin processes user signin requests for authenticated channels.
// It validates the signin payload, extracts user data, verifies the authentication token,
// associates the user ID with the socket, adds the user to the adapter, sends a success
// response, and signals that authentication is complete. This enables user-specific
// features like presence channels and user-targeted messaging.
func (w *WebSocket) handleUserSignin(msgRaw []byte) *util.Error {
	var signinPayload *UserSigninPayload
	if err := json.Unmarshal(msgRaw, &signinPayload); err != nil {
		w.errorf("Error unmarshalling signin payload: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	var userData *AuthDataUserData
	if err := json.Unmarshal([]byte(signinPayload.Data.UserData), &userData); err != nil {
		w.errorf("Error unmarshalling user data: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	if userData == nil || userData.ID == "" {
		return util.NewError(util.ErrCodeInvalidPayload, "ID must be present")
	}

	if w.signinTokenIsValid(signinPayload.Data.UserData, signinPayload.Data.Auth) {
		w.tracef("User token is valid")
		w.userID = userData.ID
		// add user to adapter
		err := w.server.Adapter.AddUser(w.app.ID, w)
		if err != nil {
			w.errorf("Error adding user: %s", err)
			return util.NewError(util.ErrCodeInvalidPayload, err.Error())
		}

		// send success response
		dataPayload := struct {
			UserData string `json:"user_data"`
		}{UserData: signinPayload.Data.UserData}
		_dataPayload, err := json.Marshal(dataPayload)
		if err != nil {
			w.errorf("Error marshalling signin success response: %s", err)
			return util.NewError(util.ErrCodeInvalidPayload)
		}
		w.Send(payloads.PayloadPack(constants.PusherSigninSuccess, string(_dataPayload)))

		w.signalAuthenticated()

		return nil
	} else {
		w.tracef("User token is invalid")
		w.closeConnection(util.ErrCodeUnauthorizedConnection)
		return util.NewError(util.ErrCodeUnauthorizedConnection)
	}
}

// parseIncomingPayload processes incoming WebSocket messages and dispatches to appropriate handlers.
// It unmarshals the message into a RequestEvent, logs metrics, and routes to the correct
// handler based on event type (ping, subscribe, unsubscribe, signin, or client events).
// This is the main message routing function for all incoming WebSocket traffic.
func (w *WebSocket) parseIncomingPayload(msgRaw []byte) *util.Error {
	// parse incoming payload
	// send socket heartbeat? is this still needed?
	// parse the incoming message
	var requestEvent payloads.RequestEvent
	err := json.Unmarshal(msgRaw, &requestEvent)
	if err != nil {
		w.errorf("Error unmarshalling message: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}
	if requestEvent.Event != constants.PusherPing {
		w.tracef("Parsed incoming payload: %s", requestEvent.Event)
	}

	var returnCode *util.Error

	switch requestEvent.Event {
	case constants.PusherPing:
		w.Send(payloads.PongPack())
	case constants.PusherSubscribe:
		returnCode = w.handleSubscribeRequest(msgRaw)
	case constants.PusherUnsubscribe:
		returnCode = w.handleUnsubscribeRequest(msgRaw)
	case constants.PusherSignin:
		returnCode = w.handleUserSignin(msgRaw)
	default:
		if util.IsClientEvent(requestEvent.Event) {
			// client sent a client event
			returnCode = w.handleClientEvent(msgRaw)
		} else {
			w.errorf("Unsupported event name: %s", requestEvent.Event)
			returnCode = util.NewError(util.ErrCodeClientEventRejected)
		}

	}
	if w.app != nil && w.app.ID != "" {
		if w.server.MetricsManager != nil {
			w.server.MetricsManager.MarkWsMessageReceived(w.app.ID, requestEvent)
		}
	}

	return returnCode
}

// resetReadTimeout updates the read deadline for the connection
// Used to implement an activity timeout mechanism
func (w *WebSocket) resetReadTimeout() {
	// reset read timeout
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(w.app.ReadTimeout))
	}
}

// sendCacheIfNeeded sends cached data if the channel is a cache channel.
// It checks if the channel has caching enabled, retrieves cached data from the cache manager,
// and either sends the cached data to the client or sends a cache_miss event if no
// cached data exists. This enables efficient data delivery for cache-enabled channels.
func (w *WebSocket) sendCacheIfNeeded(channelObj *Channel) {
	if !channelObj.IsCache {
		return
	}
	cacheKey := cache.GetChannelCacheKey(w.app.ID, channelObj.Name)
	log.Logger().Tracef("This is a cache Channel - do the needful")

	cachedData, exists := w.server.CacheManager.Get(cacheKey)
	if exists {
		w.Send([]byte(cachedData))
	} else {
		// send the cache_miss event to the client and send the cache_miss webhook
		log.Logger().Tracef("Cache miss for Channel: %s", cacheKey)
		cMiss := payloads.PayloadPack(constants.PusherCacheMiss, "")
		w.Send(cMiss)
		w.server.QueueManager.SendCacheMissed(w.app, channelObj.Name)
	}
}

// sendChannelError sends a standardized error response for channel-related errors.
// It creates a properly formatted error event and sends it to the client, providing
// consistent error handling across all channel operations.
func (w *WebSocket) sendChannelError(channel constants.ChannelName, code util.ErrorCode, message string) {
	errorEvent := payloads.ChannelEvent{
		Event: constants.PusherError,
		Data: payloads.ChannelErrorData{
			Code:    int(code),
			Message: message,
		},
		Channel: channel,
	}
	errorData := errorEvent.ToJson(true)
	w.Send(errorData)
}

// sendPresenceSubscriptionSuccess sends the subscription success response for presence channels.
// It builds a presence data structure containing all member IDs and their user info,
// marshals it to JSON, and sends it as the subscription success response. This provides
// the client with the current list of all members in the presence channel.
func (w *WebSocket) sendPresenceSubscriptionSuccess(channelObj *Channel, members map[string]*pusherClient.MemberData) {
	_presenceData := payloads.PresenceData{
		IDs:   []string{},
		Hash:  map[string]map[string]string{},
		Count: 0,
	}

	// broadcast the pusher_internal:subscription_succeeded event
	for _, member := range members {
		_presenceData.IDs = append(_presenceData.IDs, member.UserID)
		_presenceData.Hash[member.UserID] = member.UserInfo
	}
	_presenceData.Count = len(_presenceData.IDs)
	presenceData, _ := json.Marshal(map[string]payloads.PresenceData{"presence": _presenceData})

	w.Send(payloads.SubscriptionSucceededPack(channelObj.Name, string(presenceData)))
}

// sendSubscriptionError sends a standardized subscription error response.
// It creates a properly formatted subscription error and sends it to the client,
// providing consistent error handling for subscription failures.
func (w *WebSocket) sendSubscriptionError(channel constants.ChannelName, errorType, message string, code util.ErrorCode) {
	w.Send(payloads.SubscriptionErrPack(channel, errorType, message, int(code)))
}

// signalAuthenticated signals that the user has been authenticated.
// It sends a success signal through the userAuthChannel to complete the authentication
// process. This is called after successful user signin or channel authorization to
// notify the authentication watcher that the user is now authenticated.
func (w *WebSocket) signalAuthenticated() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.userAuthChannel != nil && !w.userHasAuthorized {
		// The channel is still open and user isn't authorized yet
		select {
		case w.userAuthChannel <- true:
			w.tracef("Sent auth success signal")
		default:
			w.tracef("Could not send auth signal (channel full or closed)")
		}
	}
}

// signinTokenIsValid validates the signin token using HMAC signature verification.
// It constructs the expected signature by combining the socket ID, user data, and app secret,
// then compares it with the provided signature. This ensures that only authorized users
// can sign in with valid tokens.
func (w *WebSocket) signinTokenIsValid(userData string, signatureToCheck string) bool {
	w.tracef("Checking if user token is valid")

	decodedString := fmt.Sprintf("%s::user::%s", w.ID, userData)
	w.tracef("Decoded string: %s", decodedString)

	signedString := util.HmacSignature(decodedString, w.app.Secret)
	w.tracef("Signed string: %s", signedString)

	expectedSignature := fmt.Sprintf("%s:%s", w.app.Key, signedString)
	w.tracef("Expected signature: %s", expectedSignature)

	return expectedSignature == signatureToCheck
}

// subscribeToUserChannel handles subscription to user-specific channels.
// It creates a channel name using the user ID prefix and sends a subscription success
// response. This enables direct messaging to specific users through dedicated channels.
func (w *WebSocket) subscribeToUserChannel() {
	channelName := constants.SocketRushServerToUserPrefix + w.userID
	w.Send(payloads.SubscriptionSucceededPack(channelName, "{}"))
}

// tracef logs a message at the trace level, including the socket ID for context.
// This provides consistent trace logging with socket identification for debugging.
func (w *WebSocket) tracef(format string, args ...interface{}) {
	// log error
	msg := fmt.Sprintf(format, args...)
	log.Logger().Tracef("[%s] %s", w.ID, msg)
}

// unsubscribeFromAllChannels removes the socket from all subscribed channels
// It safely gathers channels before unsubscribing to avoid deadlocks
func (w *WebSocket) unsubscribeFromAllChannels() {
	var channels []*Channel

	// within the mutex lock, copy the channels to a slice to avoid deadlocks
	w.mutex.Lock()
	for _, channel := range w.SubscribedChannels {
		channels = append(channels, channel)
	}
	w.mutex.Unlock()

	// now we can call unsubscribe on each Channel without holding the mutex lock
	for _, channel := range channels {
		w.unsubscribeFromChannel(channel)
	}
	w.SubscribedChannels = nil
}

// unsubscribeFromChannel removes the socket from a channel and handles cleanup.
// For presence channels, it manages member removal events, sends webhooks when users
// leave, broadcasts member_removed events to other subscribers, and handles channel
// vacated events when the last member leaves. It also cleans up local subscription state.
func (w *WebSocket) unsubscribeFromChannel(channelObj *Channel) {
	w.tracef("Unsubscribing from Channel: %s", channelObj.Name)
	leaveResponse := channelObj.Leave(w.server.Adapter, w)

	if leaveResponse.Success {
		w.tracef("Left Channel: %s", channelObj.Name)

		if channelObj.Type == constants.ChannelTypePresence {
			w.mutex.Lock()
			delete(w.PresenceData, channelObj.Name)
			w.mutex.Unlock()
			// get Channel members to see if the user id is still in the Channel (from a different socket)
			members := w.server.Adapter.GetChannelMembers(w.app.ID, channelObj.Name, false)

			if leaveResponse.Member == nil {
				return
			}

			if _, idExists := members[leaveResponse.Member.UserID]; !idExists {
				// user ID is no longer in this Channel; send the member_removed event and the webhook for member removed

				w.server.QueueManager.SendMemberRemoved(w.app, channelObj.Name, leaveResponse.Member.UserID)

				log.Logger().Debugf("User ID %s is no longer in Channel %s", leaveResponse.Member.UserID, channelObj.Name)
				channelEvent := payloads.ChannelEvent{
					Event:   constants.PusherInternalPresenceMemberRemoved,
					Channel: channelObj.Name,
					Data: struct {
						ID string `json:"user_id"`
					}{ID: leaveResponse.Member.UserID},
				}
				_memberRemoved := channelEvent.ToJson(true)
				_ = w.server.Adapter.Send(w.app.ID, channelObj.Name, _memberRemoved, w.ID)
			}
		}
		if leaveResponse.RemainingConnections == 0 {
			w.server.QueueManager.SendChannelVacated(w.app, channelObj.Name)
			log.Logger().Debugf("Channel %s is now empty", channelObj.Name)
		}
		w.mutex.Lock()
		delete(w.SubscribedChannels, channelObj.Name)
		w.mutex.Unlock()
	} else {
		w.errorf("Failed to leave Channel: %s", channelObj.Name)
	}
}

// validateClientEvent performs all validation checks for client events.
// It validates that client events are enabled for the app, checks event name length
// against app limits, and verifies payload size is within allowed limits. Returns
// appropriate errors and sends error responses to the client if validation fails.
func (w *WebSocket) validateClientEvent(channel constants.ChannelName, eventName string, msgRaw []byte) *util.Error {
	// Check if app has client messages enabled
	if !w.app.EnableClientMessages {
		w.sendChannelError(channel, util.ErrCodeClientEventRejected, "Client events are disabled for this app")
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	// Check if the event name is valid
	if len(eventName) > w.app.MaxEventNameLength {
		w.sendChannelError(channel, util.ErrCodeClientEventRejected, "Event name is too long")
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	// Check that the total payload of the message body (in kilobytes) is not too big
	if (len(msgRaw) / 1024) > w.app.MaxEventPayloadInKb {
		w.sendChannelError(channel, util.ErrCodeClientEventRejected, "Payload is too big")
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	return nil
}
