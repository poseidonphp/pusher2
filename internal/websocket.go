package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/apps"
	"pusher/internal/cache"

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
		// close(w.done)
	}
	w.sendMutex.Unlock()
	// reset the read timeout after a successful send, because the pusher client will reset it on receive
	w.resetReadTimeout()
}

// Close cleanly shuts down the WebSocket connection due to server shutdown
func (w *WebSocket) Close() {
	w.closeConnection(util.ErrCodeServerShuttingDown)
}

// closeConnection closes the WebSocket connection with a specific error code
// It unsubscribes from all channels and removes socket from adapter
func (w *WebSocket) closeConnection(code util.ErrorCode) {
	w.tracef("closing connection with code: %d", code)
	w.unsubscribeFromAllChannels()
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.app != nil && w.server != nil && w.server.Adapter != nil {

		_ = w.server.Adapter.RemoveSocket(w.app.ID, w.ID)
		// todo metrics mark disconnection
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
	// close(w.done)
	w.Cleanup()
}

// Listen continuously reads messages from the WebSocket connection
// It handles incoming messages and responds to client requests
func (w *WebSocket) Listen() {
	defer func() {
		w.tracef("closing listener")
		// w.closeConnection(util.ErrCodeGoRoutineExited)
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

		// parse incoming payload using msgRaw
		parseErr := w.parseIncomingPayload(msgRaw)
		if parseErr != nil {
			log.Logger().Error("failed to parse incoming payload: ", err)
			w.Send(payloads.ErrorPack(parseErr.Code, parseErr.Message))
			// do we want to close the connection here? or leave it open?
			// continue
		}
	}
}

// resetReadTimeout updates the read deadline for the connection
// Used to implement an activity timeout mechanism
func (w *WebSocket) resetReadTimeout() {
	// reset read timeout
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(w.app.ReadTimeout))
	}
}

// parseIncomingPayload processes incoming WebSocket messages
// Dispatches to appropriate handlers based on event type
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

	w.server.MetricsManager.MarkWsMessageReceived(w.app.ID, requestEvent)

	// if returnCode != nil && returnCode.Code >= 4000 && returnCode.Code <= 4299 {
	// 	w.closeConnection(returnCode.Code)
	// }
	return returnCode
}

func (w *WebSocket) subscribeToUserChannel() {
	channelName := constants.SocketRushServerToUserPrefix + w.userID

	w.Send(payloads.SubscriptionSucceededPack(channelName, "{}"))
	return

}

// handleSubscribeRequest processes a client's Channel subscription request
// It validates the Channel name, joins the Channel, and sends confirmation
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

	// Check if we are subscribing to a user channel
	if w.userID != "" && channel == constants.SocketRushServerToUserPrefix+w.userID {
		w.subscribeToUserChannel()
		return nil
	}

	channelErr := util.ValidateChannelName(channel, w.app.MaxChannelNameLength)
	if channelErr != nil {
		w.errorf("%s", channelErr)
		w.Send(payloads.ErrorPack(util.ErrCodeUnauthorizedConnection, channelErr.Error()))
		return util.NewError(util.ErrCodeInvalidChannel)
	}

	// Join the Channel
	channelObj := CreateChannelFromString(w.app, channel)

	joinResponse := channelObj.Join(w.server.Adapter, w, subscribePayload)
	if !joinResponse.Success {
		w.errorf("Failed to join Channel: %s", joinResponse.Message)
		w.Send(payloads.SubscriptionErrPack(channelObj.Name, joinResponse.Type, joinResponse.Message, int(joinResponse.ErrorCode)))
		switch joinResponse.ErrorCode {
		case util.ErrCodeSubscriptionAccessDenied:
			w.errorf("Subscription access denied: %s", joinResponse.Message)
			w.Send(payloads.SubscriptionErrPack(channelObj.Name, "AuthError", joinResponse.Message, int(joinResponse.ErrorCode)))
			return util.NewError(util.ErrCodeSubscriptionAccessDenied)
		default:
			w.errorf("Failed to join Channel: %s", joinResponse.Message)
			w.Send(payloads.SubscriptionErrPack(channelObj.Name, "Unknown", joinResponse.Message, int(joinResponse.ErrorCode)))
			return util.NewError(util.ErrCodeGenericReconnect)
		}
	}

	// Check if Channel exists in SubscribedChannels
	w.mutex.Lock()
	if _, ok := w.SubscribedChannels[channel]; !ok {
		w.SubscribedChannels[channelObj.Name] = channelObj
	}
	w.mutex.Unlock()

	// Check if channel authorization is required
	w.mutex.Lock()
	if channelObj.RequiresAuth && !w.userHasAuthorized && w.userAuthChannel != nil {
		// check if the user id is set on the channel response, and if the socket user id is not already set
		// we will not overwrite the user ID if it was set by the sign-in method
		if w.userID == "" && joinResponse.Member != nil && joinResponse.Member.UserID != "" {
			w.userID = joinResponse.Member.UserID
		}
		w.mutex.Unlock()

		w.signalAuthenticated()
	} else {
		w.mutex.Unlock()
	}

	// line 384 (ws-handler.ts), do i need to do this?

	if joinResponse.ChannelConnections == 1 {
		w.server.QueueManager.SendChannelOccupied(w.app, channelObj.Name)
		log.Logger().Debug("Channel occupied")
	}

	// for non-presence channels, end with subscription succeeded
	if channelObj.Type != constants.ChannelTypePresence {
		w.Send(payloads.SubscriptionSucceededPack(channelObj.Name, "{}"))
		w.sendCacheIfNeeded(channelObj)
		return nil
	}

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
	w.mutex.Lock()
	if _, exists := members[joinResponse.Member.UserID]; !exists {
		w.mutex.Unlock()
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
	} else {
		w.mutex.Unlock()
		w.tracef("Member already exists, not sending member added event")
	}

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
	w.sendCacheIfNeeded(channelObj)

	return nil
}

func (w *WebSocket) sendCacheIfNeeded(channelObj *Channel) {
	if !channelObj.IsCache {
		return
	}
	cacheKey := cache.GetChannelCacheKey(w.app.ID, channelObj.Name)
	log.Logger().Tracef("This is a cache Channel - do the needful")
	// 		// retrieve the data from the cache if it exists, otherwise send the cache_miss event
	cachedData, exists := w.server.CacheManager.Get(cacheKey)
	if exists {
		w.Send([]byte(cachedData))
	} else {
		// send the cache_miss event to the client and send the cache_miss webhook
		log.Logger().Tracef("Cache miss for Channel: %s", cacheKey)

		// 	send the cache_miss event to the client and send the cache_miss webhook
		cMiss := payloads.PayloadPack(constants.PusherCacheMiss, "")
		w.Send(cMiss)
		w.server.QueueManager.SendCacheMissed(w.app, channelObj.Name)
	}

}

// handleUnsubscribeRequest processes a client's Channel unsubscription request
// It validates the request and calls unsubscribeFromChannel
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

// unsubscribeFromChannel removes the socket from a Channel
// For presence channels, it also handles member removal events
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

// handleClientEvent processes client-generated events
// Validates permissions and broadcasts the event to other subscribers
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
		e := payloads.ChannelEvent{
			Event: constants.PusherError,
			Data: payloads.ChannelErrorData{
				Code:    int(util.ErrCodeClientEventRejected),
				Message: fmt.Sprintf("Client is not subscribed to %s", channel),
			},
			Channel: channel,
		}
		eData := e.ToJson(true)
		w.Send(eData)
		return nil
	}
	w.mutex.Unlock()

	// Check if app has client messages enabled
	if !w.app.EnableClientMessages {
		e := payloads.ChannelEvent{
			Event: constants.PusherError,
			Data: payloads.ChannelErrorData{
				Code:    int(util.ErrCodeClientEventRejected),
				Message: "Client events are disabled for this app",
			},
			Channel: channel,
		}
		eData := e.ToJson(true)
		w.Send(eData)
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	// Check if the event name is valid
	if len(clientChannelEvent.Event) > w.app.MaxEventNameLength {
		e := payloads.ChannelEvent{
			Event: constants.PusherError,
			Data: payloads.ChannelErrorData{
				Code:    int(util.ErrCodeClientEventRejected),
				Message: "Event name is too long",
			},
			Channel: channel,
		}
		eData := e.ToJson(true)
		w.Send(eData)
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	// Check that the total payload of the message body (in kiloybtes) is not too big
	if (len(msgRaw) / 1024) > w.app.MaxEventPayloadInKb {
		e := payloads.ChannelEvent{
			Event: constants.PusherError,
			Data: payloads.ChannelErrorData{
				Code:    int(util.ErrCodeClientEventRejected),
				Message: "Payload is too big",
			},
			Channel: channel,
		}
		eData := e.ToJson(true)
		w.Send(eData)
		return util.NewError(util.ErrCodeClientEventRejected)
	}

	// check to make sure the client is in the channel. Because this is a client event, we can force a onlyLocal check
	if !w.server.Adapter.IsInChannel(w.app.ID, channel, w.ID, true) {
		w.tracef("Socket %s is not in Channel %s", w.ID, channel)
		return nil
	}

	// TODO wrap in rateLimiter (ws-handler.ts: 594)
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

// handleReadMessageError processes WebSocket read errors
// Differentiates between expected and unexpected closure events
func (w *WebSocket) handleReadMessageError(err error) {
	expectedCodes := []int{websocket.CloseGoingAway, websocket.CloseNormalClosure}
	if websocket.IsUnexpectedCloseError(err, expectedCodes...) {
		var e *websocket.CloseError
		if errors.As(err, &e) {
			w.errorf("Unexpected close error: %s", e.Error())
		} else {
			w.errorf("Unexpected close error: %s", err)
		}
		w.closeConnection(util.ErrCodeWebsocketAbnormalClosure)
	} else {
		w.tracef("Closing connection as expected: %s", err.Error())
		w.closeConnection(util.ErrCodeCloseExpected)
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

func (w *WebSocket) handleUserSignin(msgRaw []byte) *util.Error {

	var signinPayload *UserSigninPayload
	_ = json.Unmarshal(msgRaw, &signinPayload)

	var userData *AuthDataUserData
	_ = json.Unmarshal([]byte(signinPayload.Data.UserData), &userData)

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
		_dataPayload, _ := json.Marshal(dataPayload)
		w.Send(payloads.PayloadPack(constants.PusherSigninSuccess, string(_dataPayload)))

		w.signalAuthenticated()

		return nil
	} else {
		w.tracef("User token is invalid")
		w.closeConnection(util.ErrCodeUnauthorizedConnection)
		return util.NewError(util.ErrCodeUnauthorizedConnection)
	}
}

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

// errorf logs a message at the error level, including the socket ID.
func (w *WebSocket) errorf(format string, args ...interface{}) {
	// log error
	msg := fmt.Sprintf(format, args...)
	log.Logger().Errorf("[%s] %s", w.ID, msg)
}

// tracef logs a message at the trace level, including the socket ID.
func (w *WebSocket) tracef(format string, args ...interface{}) {
	// log error
	msg := fmt.Sprintf(format, args...)
	log.Logger().Tracef("[%s] %s", w.ID, msg)
}

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

	// if w.server != nil {
	// 	w.server.websocketPool.Put(w)
	// }
}
