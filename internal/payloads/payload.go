package payloads

import (
	"encoding/json"
	"pusher/env"
	"pusher/internal/constants"
	"pusher/internal/util"
	"pusher/log"
)

// RequestEvent ...
type RequestEvent struct {
	Event string `json:"event"`
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
		panic(err)
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
func EstablishPack(socketID constants.SocketID) []byte {
	data := EstablishData{SocketID: socketID, ActivityTimeout: env.GetInt("ACTIVITY_TIMEOUT", 10)}
	//data := EstablishData{SocketID: socketID, ActivityTimeout: 20}
	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return PayloadPack(constants.PusherConnectionEstablished, string(b[:]))
}

// **** ERROR DATA ****

// ErrData Channels -> Client
type ErrData struct {
	Message string         `json:"message"`
	Code    util.ErrorCode `json:"code,omitempty"` // optional
}

// ErrPack Channels -> Client
func ErrPack(code util.ErrorCode) []byte {
	data := ErrData{Message: util.ErrCodes[code], Code: code}

	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return PayloadPack(constants.PusherError, string(b))
}

// **** PONG ****

type Pong struct {
	Event string `json:"event"`
}

func PongPack() []byte {
	data := Pong{Event: constants.PusherPong}

	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return b
}

// **** SUBSCRIBE ****

// Subscribe ...
type SubscribePayload struct {
	Event string               `json:"event"`
	Data  SubscribeChannelData `json:"data"`
}

// SubscribeData ...
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
		panic(err)
	}
	return b
}

// PresenceData ...
type PresenceData struct {
	IDs  []string                     `json:"ids"`
	Hash map[string]map[string]string `json:"hash"`
	//Hash  map[string]map[string]string `json:"hash"`
	Count int `json:"count"`
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

func MarshalEventPack(payload any) []byte {
	b, err := json.Marshal(payload)
	if err != nil {
		log.Logger().Errorf("Error marshalling event pack: %s", err.Error())
		panic(err)
	}
	return b
}
