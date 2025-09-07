package payloads

import (
	"encoding/json"

	"pusher/internal/constants"
	"pusher/internal/util"
	"pusher/log"
)

// RequestEvent ...
type RequestEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Payload ...
type Payload struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// PayloadPack websocket payload to send to client
func PayloadPack(event, data string) []byte {
	payload := Payload{Event: event, Data: data}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Logger().Errorf("Error marshalling Payload in PayloadPack: %s", err)
		return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing event data")
	}
	return payloadJSON
}

// **** NEW CONNECTION ESTABLISHED ****

// EstablishData Channels -> Client
type EstablishData struct {
	SocketID        constants.SocketID `json:"socket_id"`
	ActivityTimeout int                `json:"activity_timeout"` // Protocol 7 and above
}

// EstablishPack Channels -> Client
func EstablishPack(socketID constants.SocketID, timeout int) []byte {
	data := EstablishData{SocketID: socketID, ActivityTimeout: timeout}
	// data := EstablishData{SocketID: socketID, ActivityTimeout: 20}
	b, err := json.Marshal(data)
	if err != nil {
		log.Logger().Errorf("Error marshalling EstablishData in EstablishPack: %s", err)
		return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing event data")
	}
	return PayloadPack(constants.PusherConnectionEstablished, string(b[:]))
}

// **** ERROR DATA ****

// ErrData Channels -> Client
type ErrData struct {
	Message string         `json:"message"`
	Code    util.ErrorCode `json:"code,omitempty"` // optional
}

// **** PONG ****

type Pong struct {
	Event string `json:"event"`
}

func PongPack() []byte {
	data := Pong{Event: constants.PusherPong}

	b, err := json.Marshal(data)
	if err != nil {
		log.Logger().Errorf("Error marshalling Pong in PongPack: %s", err)
		return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing event data")
	}
	return b
}

// **** SUBSCRIBE ****

// SubscribePayload ...
type SubscribePayload struct {
	Event string               `json:"event"`
	Data  SubscribeChannelData `json:"data"`
}

// SubscribeChannelData ...
type SubscribeChannelData struct {
	Channel     constants.ChannelName `json:"channel"`
	Auth        string                `json:"auth,omitempty"`         // optional
	ChannelData string                `json:"channel_data,omitempty"` // optional PresenceChannelData
}

// UnsubscribePayload ...
type UnsubscribePayload struct {
	Event string          `json:"event"`
	Data  UnsubscribeData `json:"data"`
}

// UnsubscribeData ...
type UnsubscribeData struct {
	Channel constants.ChannelName `json:"channel"`
}

// SubscriptionSucceeded CHANNELS -> CLIENT
// @doc https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol#presence-channel-events
type SubscriptionSucceeded struct {
	Event   string                `json:"event"`
	Channel constants.ChannelName `json:"channel"`
	Data    string                `json:"data"`
}

// SubscriptionSucceededPack ...
func SubscriptionSucceededPack(channel constants.ChannelName, data string) []byte {
	d := SubscriptionSucceeded{Event: constants.PusherInternalSubscriptionSucceeded, Channel: channel, Data: data}

	b, err := json.Marshal(d)
	if err != nil {
		log.Logger().Errorf("Error marshalling SubscriptionSucceeded in SubscriptionSucceededPack: %s", err)
		return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing SubscriptionSucceeded data")
	}
	return b
}

type ErrorData struct {
	Type   string `json:"type"`
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// SubscriptionErrPack ...
func SubscriptionErrPack(channelName constants.ChannelName, errType string, message string, code int) []byte {
	d := ErrorData{Type: errType, Error: message, Status: code}
	_d, _ := json.Marshal(d)
	data := &SubscriptionSucceeded{
		Event:   constants.PusherSubscriptionError,
		Channel: channelName,
		Data:    string(_d),
	}
	b, _ := json.Marshal(data)
	return b
}

// PresenceData ...
type PresenceData struct {
	IDs   []string                     `json:"ids"`
	Hash  map[string]map[string]string `json:"hash"`
	Count int                          `json:"count"`
}

type MemberAddedPack struct {
	Event   string                `json:"event"`
	Data    string                `json:"data"`
	Channel constants.ChannelName `json:"channel"`
}

// ClientChannelEvent ...
type ClientChannelEvent struct {
	Event   string                `json:"event"`
	Data    json.RawMessage       `json:"data"`
	Channel constants.ChannelName `json:"channel"`
}

// ChannelCacheMiss ...
type ChannelCacheMiss struct {
}

type ChannelEvent struct {
	Event    string                `json:"event"`
	Data     any                   `json:"data,omitempty"`
	Channel  constants.ChannelName `json:"channel,omitempty"`
	UserID   string                `json:"user_id,omitempty"`   // optional, present only if this is a `client event` on a `presence channel`
	SocketID constants.SocketID    `json:"socket_id,omitempty"` // optional, skips the event from being sent to this socket
}

func (ce *ChannelEvent) ToJson(marshalDataProperty bool) []byte {
	if marshalDataProperty {
		data, err := json.Marshal(ce.Data)
		if err != nil {
			log.Logger().Errorf("Error marshalling ChannelEvent data: %s", err)
			return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing event data", ce.Channel)
		}
		ce.Data = string(data)
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		log.Logger().Errorf("Error marshalling ChannelEvent: %s", err)
		return ErrorPack(util.ErrCodeInvalidPayload, "Error serializing event data", ce.Channel)
	}
	return payload
}

type ChannelErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorPack creates a JSON payload for an error event ready to be sent to the client.
// It includes the error code and message, and optionally the channel name.
// If it fails to marshal the data, it returns a predefined error message that is client-ready.
func ErrorPack(code util.ErrorCode, message string, channel ...constants.ChannelName) []byte {
	_data := ChannelErrorData{Code: int(code), Message: message}
	// data, err := json.Marshal(_data)
	// if err != nil {
	// 	log.Logger().Errorf("Error marshalling ChannelEvent data: %s", err)
	// 	return []byte(`{"event":"pusher:error","data":"{\"code\":4310,\"message\":\"Error serializing error data\"}"}`)
	// }

	ch := ""
	if len(channel) > 0 {
		ch = string(channel[0])
	}

	_dataJson, err := json.Marshal(_data)
	if err != nil {
		return getFallbackSerializationError()
	}

	_evt := ChannelEvent{
		Event:   constants.PusherError,
		Channel: ch,
		Data:    string(_dataJson),
		// Data:    string(data),
	}

	evt, err := json.Marshal(_evt)
	if err != nil {
		log.Logger().Errorf("Error marshalling ChannelEvent: %s", err)
		// Return pre-defined error message for event serialization error
		if len(channel) > 0 {
			return []byte(`{"event":"pusher:error","channel":"` + string(channel[0]) + `","data":"{\"code\":4310,\"message\":\"Error serializing error data\"}"}`)
		}
		return getFallbackSerializationError()
	}

	return evt
}

func getFallbackSerializationError() []byte {
	return []byte(`{"event":"pusher:error","data":"{\"code\":4310,\"message\":\"Error serializing error data\"}"}`)
}

type PusherApiMessage struct {
	Name     string                  `json:"name,omitempty" form:"name" binding:"required"`
	Data     string                  `json:"data,omitempty" form:"data" binding:"required"`
	Channel  constants.ChannelName   `json:"channel,omitempty" form:"channel,omitempty" binding:"required_without=Channels"`
	Channels []constants.ChannelName `json:"channels,omitempty" form:"channels,omitempty" binding:"required_without=Channel"`
	SocketID constants.SocketID      `json:"socket_id,omitempty"`
}
