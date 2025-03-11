package internal

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/storage"
	"pusher/internal/util"
	"pusher/log"
	"sync"
	"time"
)

type Session struct {
	conn             *websocket.Conn
	client           string
	version          string
	protocol         int
	sendChannel      chan []byte // Buffered channel of outbound messages.
	mutex            sync.Mutex
	subscriptions    map[constants.ChannelName]bool // map of channel names this session is subscribed to
	presenceChannels map[constants.ChannelName]constants.ChannelName
	socketID         constants.SocketID
	closed           bool
	done             chan struct{} // when closed, will signal to other goroutines to exit
}

// readerSubProcess reads incoming messages from the frontend client
func (s *Session) readerSubProcess() {
	defer func() {
		log.Logger().Tracef("Closing readMessagesFromConnection() for %s", s.socketID)
		s.closeConnection(util.ErrCodeGoRoutineExited)
	}()

	s.conn.SetReadLimit(constants.MaxMessageSize)

	// Create a blocking loop that continuously reads messages from the websocket connection
	for {
		_, msgRaw, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				var e *websocket.CloseError
				if errors.As(err, &e) {
					log.Logger().Errorf("Unexpected close error: %s", e.Error())
				} else {
					log.Logger().Errorf("Unexpected close error: %s", err)
				}
				s.closeConnection(util.ErrCodeWebsocketAbnormalClosure)
			} else {
				log.Logger().Tracef("Closing connection as expected: %s", err.Error())
				s.closeConnection(util.ErrCodeCloseExpected)
			}
			break // exit the loop
		}

		// We received a message from the client; reset the read timeout
		s.resetReadTimeout()

		// Parse the incoming payload
		parseErr := s.ParseIncomingPayload(msgRaw)
		if parseErr != nil {
			log.Logger().Errorf("Error parsing incoming payload: %s", parseErr)
			s.closeConnection(util.ErrCodeInvalidPayload)
			break // exit the loop
		}

		// Just in case it needs it, let the storage manager know that this client is still alive
		_ = storage.Manager.SocketDidHeartbeat(GlobalHub.nodeID, s.socketID, s.presenceChannels)
	}
}

// senderSubProcess sends a message to the client
func (s *Session) senderSubProcess() {
	// run the ticker, ping the client every x seconds
	ticker := time.NewTicker(config.ActivityTimeout)
	defer func() {
		ticker.Stop()
		log.Logger().Traceln("Closing senderSubProcess() for", s.socketID)
		GlobalHub.unregister <- s
		s.closeConnection(util.ErrCodeGoRoutineExited)
	}()

	// create a blocking loop for sending messages to the client, pinging for heartbeat, and handling the done/closure channel
	for {
		select {
		case message, ok := <-s.sendChannel:
			if !ok {
				// the send channel was closed
				return
			}
			err := s.write(message)
			if err != nil {
				log.Logger().Tracef("Error writing message to client: %s", err)
				return
			}
		case <-s.done:
			log.Logger().Tracef("Closing senderSubProcess() as a result of done channel for %s", s.socketID)
			return
		case <-ticker.C:
			// send a ping to the client
			if s.conn == nil {
				return
			}
			_ = s.conn.SetWriteDeadline(time.Now().Add(constants.WriteWait))
			s.Send(payloads.PingPongPack(constants.PusherPing))
			s.resetPongTimeout()
		}
	}
}

// Send a message to the client
func (s *Session) Send(msg []byte) {
	if s.sendChannel == nil || s.conn == nil || s.closed {
		return
	}

	// use the select statement to ensure this is non-blocking
	select {
	case s.sendChannel <- msg:
	default:
		// The send channel is full, close it and set to nil
		if s.sendChannel != nil {
			close(s.sendChannel)
		}
		s.sendChannel = nil
	}
}

func (s *Session) write(message []byte) error {
	if s.conn == nil {
		return errors.New("connection is nil")
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(constants.WriteWait))
	w, err := s.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		log.Logger().Errorf("Error getting next writer: %s", err)
		return err
	}
	_, _ = w.Write(message)
	return w.Close()
}

func (s *Session) ParseIncomingPayload(msgRaw []byte) *util.Error {
	// parse the incoming message
	var requestEvent payloads.RequestEvent
	err := json.Unmarshal(msgRaw, &requestEvent)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling message: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}

	switch requestEvent.Event {
	case constants.PusherPing:
		// client sent a ping, respond with a pong
		s.Send(payloads.PingPongPack(constants.PusherPong))
	case constants.PusherPong:
		// client sent a pong, don't need to do anything, the timeout was already reset when message was received
		return nil
	case constants.PusherSubscribe:
		// client wants to subscribe to a channel
		return s.handleSubscribeRequest(msgRaw)
	case constants.PusherUnsubscribe:
		// client wants to unsubscribe from a channel
		return s.handleUnsubscribeRequest(msgRaw)
	default:
		if util.IsClientEvent(requestEvent.Event) {
			// client sent a client event
			return s.handleClientEvent(msgRaw)
		}
		log.Logger().Errorf("Unsupported event name: %s", requestEvent.Event)
	}
	return nil
}

// handleUnsubscribeRequest handles a client request to unsubscribe from a channel
func (s *Session) handleUnsubscribeRequest(msgRaw []byte) *util.Error {
	var unsubscribePayload payloads.UnsubscribePayload
	err := json.Unmarshal(msgRaw, &unsubscribePayload)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling unsubscribe payload: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}
	channel := unsubscribePayload.Data.Channel
	if !util.ValidChannel(channel) {
		log.Logger().Errorf("Invalid channel name: %s", channel)
		return util.NewError(util.ErrCodeInvalidChannel)
	}

	// Remove from session subscription list
	s.mutex.Lock()
	if !s.subscriptions[channel] {
		s.mutex.Unlock()
		return util.NewError(util.ErrCodeNotSubscribed)
	}
	delete(s.subscriptions, channel)
	if _, ok := s.presenceChannels[channel]; ok {
		delete(s.presenceChannels, channel)
	}
	s.mutex.Unlock()

	// Remove from hub
	GlobalHub.removeSessionFromChannels(s, channel)
	return nil
}

// handleSubscribeRequest handles a client request to subscribe to a channel
func (s *Session) handleSubscribeRequest(msgRaw []byte) *util.Error {
	var subscribePayload payloads.SubscribePayload
	err := json.Unmarshal(msgRaw, &subscribePayload)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling subscribe payload: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload)
	}
	channel := subscribePayload.Data.Channel
	if !util.ValidChannel(channel) {
		log.Logger().Errorf("Invalid channel name: %s", channel)
		s.Send(payloads.ErrPack(util.ErrCodeInvalidChannel))
		return util.NewError(util.ErrCodeInvalidChannel)
	}

	s.mutex.Lock()
	if s.subscriptions[channel] {
		s.mutex.Unlock()
		s.Send(payloads.ErrPack(util.ErrCodeAlreadySubscribed))
		return util.NewError(util.ErrCodeAlreadySubscribed)
	}
	s.mutex.Unlock()

	ch := GlobalHub.getOrCreateLocalChannel(channel)

	// Validate the auth token if it's a private or presence channel
	if ch.Type == constants.ChannelTypePrivate || ch.Type == constants.ChannelTypePresence || ch.Type == constants.ChannelTypePrivateEncrypted {
		authToken := subscribePayload.Data.Auth
		if !util.ValidateChannelAuth(authToken, s.socketID, channel, subscribePayload.Data.ChannelData) {
			log.Logger().Errorf("Invalid auth token for channel: %s", channel)
			// send an error message to the client
			errPack := payloads.ErrPack(util.ErrCodeUnauthorizedConnection)
			s.Send(errPack)
			return util.NewError(util.ErrCodeUnauthorizedConnection)
		}
	}

	// Made it this far, good to continue subscribing the user
	if util.IsPresenceChannel(channel) {
		// get the member data for currently connected users
		memberData, presenceErr := ValidatePresenceChannelRequirements(channel, subscribePayload.Data.ChannelData)
		if presenceErr != nil {
			s.Send(payloads.ErrPack(presenceErr.Code))
			return presenceErr
		}
		s.mutex.Lock()
		s.presenceChannels[channel] = channel
		s.mutex.Unlock()
		s.confirmedChannelSubscription(ch, &memberData)
	} else {
		s.confirmedChannelSubscription(ch, nil)
	}
	return nil
}

func (s *Session) addChannelToSubscriptions(channelName constants.ChannelName) {
	s.mutex.Lock()
	s.subscriptions[channelName] = true
	s.mutex.Unlock()
}

func (s *Session) removeChannelFromSubscriptions(channel constants.ChannelName) {
	s.mutex.Lock()
	if s.subscriptions[channel] {
		delete(s.subscriptions, channel)
	}
	s.mutex.Unlock()
}

func (s *Session) confirmedChannelSubscription(channel *Channel, memberData *pusherClient.MemberData) {
	// add the channel to the sessions subscriptions
	s.addChannelToSubscriptions(channel.Name)

	// add the user to the presence channel data on the hub
	if channel.Type == constants.ChannelTypePresence {
		if memberData == nil {
			return
		}

		GlobalHub.addPresenceUser(s.socketID, *channel, *memberData)

		// Get the current presence channel data from the hub, send to the user
		presenceData, pErr := storage.Manager.GetPresenceData(channel.Name)

		if pErr != nil {
			log.Logger().Errorf("Error marshalling presence data: %s", pErr)
			s.removeChannelFromSubscriptions(channel.Name)
			return
		}
		// send the client the subscription confirmation
		s.Send(payloads.SubscriptionSucceededPack(channel.Name, string(presenceData)))
	} else {
		s.Send(payloads.SubscriptionSucceededPack(channel.Name, "{}"))
	}

	// add the session to the channel on the hub
	GlobalHub.addSessionToChannel(s, channel)
}

func (s *Session) handleClientEvent(msgRaw []byte) *util.Error {
	var clientChannelEvent payloads.ClientChannelEvent
	err := json.Unmarshal(msgRaw, &clientChannelEvent)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling client event: %s", err)
		return util.NewError(util.ErrCodeInvalidPayload, err.Error())
	}

	channel := clientChannelEvent.Channel

	// Check if session is subscribed to channel
	s.mutex.Lock()
	if !s.subscriptions[channel] {
		s.mutex.Unlock()
		log.Logger().Errorf("Client event for unsubscribed channel: %s", channel)
		return util.NewError(util.ErrCodeNotSubscribed)
	}
	s.mutex.Unlock()

	if !util.IsPresenceChannel(channel) && !util.IsPrivateChannel(channel) {
		return util.NewError(util.ErrCodeClientOnlySupportsPrivatePresence)
	}

	var eventForm ChannelEvent
	eventForm.Channel = channel
	eventForm.Event = clientChannelEvent.Event
	eventForm.SocketID = s.socketID // exclude
	dataBytes, marshalErr := clientChannelEvent.Data.MarshalJSON()
	if marshalErr != nil {
		return util.NewError(util.ErrCodeInvalidPayload, marshalErr.Error())
	}
	eventForm.Data = string(dataBytes)

	if util.IsPresenceChannel(channel) {
		//presenceChannelData, getPresenceErr := GlobalHub.getPresenceMemberDataForSocket(channel, s.socketID)
		presenceChannelData, getPresenceErr := storage.Manager.GetPresenceDataForSocket(GlobalHub.nodeID, channel, s.socketID)
		if getPresenceErr != nil {
			return util.NewUnknownError(getPresenceErr.Error())
		}
		//hookEvent.UserID = presenceChannelData.UserID

		eventForm.UserID = presenceChannelData.UserID
	}
	_ = GlobalHub.PublishChannelEventGlobally(eventForm)

	return nil
}

func (s *Session) resetReadTimeout() {
	// set the read deadline
	e := s.conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	if e != nil {
		log.Logger().Errorf("Error setting read deadline: %s", e)
	}
}

func (s *Session) resetPongTimeout() {
	_ = s.conn.SetReadDeadline(time.Now().Add(config.PongTimeout))
}

func (s *Session) closeConnection(errorCode util.ErrorCode) {
	// if the client connection is still open, close it
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}

	// if the session struct is already 'closed', return
	if s.closed {
		return
	}

	s.closed = true
	close(s.done)

	// call the removeSessionFromChannels
	GlobalHub.removeSessionFromChannels(s)
	GlobalHub.unregister <- s
}
