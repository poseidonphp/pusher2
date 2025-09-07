package payloads

import (
	"encoding/json"
	"testing"

	"pusher/internal/constants"
	"pusher/internal/util"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// REQUEST EVENT TESTS
// ============================================================================

func TestRequestEvent(t *testing.T) {
	// Test RequestEvent struct
	req := RequestEvent{
		Event: "test-event",
		Data:  "test-data",
	}

	assert.Equal(t, "test-event", req.Event)
	assert.Equal(t, "test-data", req.Data)
}

func TestRequestEventWithMapData(t *testing.T) {
	// Test RequestEvent with map data
	data := map[string]interface{}{
		"user_id": "123",
		"message": "hello",
	}

	req := RequestEvent{
		Event: "test-event",
		Data:  data,
	}

	assert.Equal(t, "test-event", req.Event)
	assert.Equal(t, data, req.Data)
}

// ============================================================================
// PAYLOAD TESTS
// ============================================================================

func TestPayload(t *testing.T) {
	// Test Payload struct
	payload := Payload{
		Event: "test-event",
		Data:  "test-data",
	}

	assert.Equal(t, "test-event", payload.Event)
	assert.Equal(t, "test-data", payload.Data)
}

func TestPayloadPack(t *testing.T) {
	// Test PayloadPack function
	event := "test-event"
	data := "test-data"

	result := PayloadPack(event, data)

	// Unmarshal to verify structure
	var payload Payload
	err := json.Unmarshal(result, &payload)
	assert.NoError(t, err)
	assert.Equal(t, event, payload.Event)
	assert.Equal(t, data, payload.Data)
}

func TestPayloadPackEmptyData(t *testing.T) {
	// Test PayloadPack with empty data
	event := "test-event"
	data := ""

	result := PayloadPack(event, data)

	var payload Payload
	err := json.Unmarshal(result, &payload)
	assert.NoError(t, err)
	assert.Equal(t, event, payload.Event)
	assert.Equal(t, data, payload.Data)
}

// ============================================================================
// ESTABLISH DATA TESTS
// ============================================================================

func TestEstablishData(t *testing.T) {
	// Test EstablishData struct
	socketID := constants.SocketID("test-socket-123")
	timeout := 30

	establishData := EstablishData{
		SocketID:        socketID,
		ActivityTimeout: timeout,
	}

	assert.Equal(t, socketID, establishData.SocketID)
	assert.Equal(t, timeout, establishData.ActivityTimeout)
}

func TestEstablishPack(t *testing.T) {
	// Test EstablishPack function
	socketID := constants.SocketID("test-socket-123")
	timeout := 30

	result := EstablishPack(socketID, timeout)

	// Unmarshal to verify structure
	var payload Payload
	err := json.Unmarshal(result, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherConnectionEstablished, payload.Event)

	// Unmarshal the data field
	var establishData EstablishData
	err = json.Unmarshal([]byte(payload.Data), &establishData)
	assert.NoError(t, err)
	assert.Equal(t, socketID, establishData.SocketID)
	assert.Equal(t, timeout, establishData.ActivityTimeout)
}

func TestEstablishPackWithZeroTimeout(t *testing.T) {
	// Test EstablishPack with zero timeout
	socketID := constants.SocketID("test-socket-123")
	timeout := 0

	result := EstablishPack(socketID, timeout)

	var payload Payload
	err := json.Unmarshal(result, &payload)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherConnectionEstablished, payload.Event)

	var establishData EstablishData
	err = json.Unmarshal([]byte(payload.Data), &establishData)
	assert.NoError(t, err)
	assert.Equal(t, socketID, establishData.SocketID)
	assert.Equal(t, timeout, establishData.ActivityTimeout)
}

// ============================================================================
// ERROR DATA TESTS
// ============================================================================

func TestErrData(t *testing.T) {
	// Test ErrData struct
	message := "Test error message"
	code := util.ErrCodeInvalidPayload

	errData := ErrData{
		Message: message,
		Code:    code,
	}

	assert.Equal(t, message, errData.Message)
	assert.Equal(t, code, errData.Code)
}

func TestErrDataWithoutCode(t *testing.T) {
	// Test ErrData without code
	message := "Test error message"

	errData := ErrData{
		Message: message,
	}

	assert.Equal(t, message, errData.Message)
	assert.Equal(t, util.ErrorCode(0), errData.Code)
}

// ============================================================================
// PONG TESTS
// ============================================================================

func TestPong(t *testing.T) {
	// Test Pong struct
	pong := Pong{
		Event: constants.PusherPong,
	}

	assert.Equal(t, constants.PusherPong, pong.Event)
}

func TestPongPack(t *testing.T) {
	// Test PongPack function
	result := PongPack()

	var pong Pong
	err := json.Unmarshal(result, &pong)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherPong, pong.Event)
}

// ============================================================================
// SUBSCRIBE PAYLOAD TESTS
// ============================================================================

func TestSubscribePayload(t *testing.T) {
	// Test SubscribePayload struct
	channel := constants.ChannelName("test-channel")
	auth := "test-auth"
	channelData := "test-channel-data"

	subscribePayload := SubscribePayload{
		Event: constants.PusherSubscribe,
		Data: SubscribeChannelData{
			Channel:     channel,
			Auth:        auth,
			ChannelData: channelData,
		},
	}

	assert.Equal(t, constants.PusherSubscribe, subscribePayload.Event)
	assert.Equal(t, channel, subscribePayload.Data.Channel)
	assert.Equal(t, auth, subscribePayload.Data.Auth)
	assert.Equal(t, channelData, subscribePayload.Data.ChannelData)
}

func TestSubscribeChannelData(t *testing.T) {
	// Test SubscribeChannelData struct
	channel := constants.ChannelName("test-channel")
	auth := "test-auth"
	channelData := "test-channel-data"

	subscribeChannelData := SubscribeChannelData{
		Channel:     channel,
		Auth:        auth,
		ChannelData: channelData,
	}

	assert.Equal(t, channel, subscribeChannelData.Channel)
	assert.Equal(t, auth, subscribeChannelData.Auth)
	assert.Equal(t, channelData, subscribeChannelData.ChannelData)
}

func TestSubscribeChannelDataWithoutOptionalFields(t *testing.T) {
	// Test SubscribeChannelData without optional fields
	channel := constants.ChannelName("test-channel")

	subscribeChannelData := SubscribeChannelData{
		Channel: channel,
	}

	assert.Equal(t, channel, subscribeChannelData.Channel)
	assert.Equal(t, "", subscribeChannelData.Auth)
	assert.Equal(t, "", subscribeChannelData.ChannelData)
}

// ============================================================================
// UNSUBSCRIBE PAYLOAD TESTS
// ============================================================================

func TestUnsubscribePayload(t *testing.T) {
	// Test UnsubscribePayload struct
	channel := constants.ChannelName("test-channel")

	unsubscribePayload := UnsubscribePayload{
		Event: constants.PusherUnsubscribe,
		Data: UnsubscribeData{
			Channel: channel,
		},
	}

	assert.Equal(t, constants.PusherUnsubscribe, unsubscribePayload.Event)
	assert.Equal(t, channel, unsubscribePayload.Data.Channel)
}

func TestUnsubscribeData(t *testing.T) {
	// Test UnsubscribeData struct
	channel := constants.ChannelName("test-channel")

	unsubscribeData := UnsubscribeData{
		Channel: channel,
	}

	assert.Equal(t, channel, unsubscribeData.Channel)
}

// ============================================================================
// SUBSCRIPTION SUCCEEDED TESTS
// ============================================================================

func TestSubscriptionSucceeded(t *testing.T) {
	// Test SubscriptionSucceeded struct
	channel := constants.ChannelName("test-channel")
	data := "test-data"

	subscriptionSucceeded := SubscriptionSucceeded{
		Event:   constants.PusherInternalSubscriptionSucceeded,
		Channel: channel,
		Data:    data,
	}

	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, subscriptionSucceeded.Event)
	assert.Equal(t, channel, subscriptionSucceeded.Channel)
	assert.Equal(t, data, subscriptionSucceeded.Data)
}

func TestSubscriptionSucceededPack(t *testing.T) {
	// Test SubscriptionSucceededPack function
	channel := constants.ChannelName("test-channel")
	data := "test-data"

	result := SubscriptionSucceededPack(channel, data)

	var subscriptionSucceeded SubscriptionSucceeded
	err := json.Unmarshal(result, &subscriptionSucceeded)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, subscriptionSucceeded.Event)
	assert.Equal(t, channel, subscriptionSucceeded.Channel)
	assert.Equal(t, data, subscriptionSucceeded.Data)
}

func TestSubscriptionSucceededPackEmptyData(t *testing.T) {
	// Test SubscriptionSucceededPack with empty data
	channel := constants.ChannelName("test-channel")
	data := ""

	result := SubscriptionSucceededPack(channel, data)

	var subscriptionSucceeded SubscriptionSucceeded
	err := json.Unmarshal(result, &subscriptionSucceeded)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherInternalSubscriptionSucceeded, subscriptionSucceeded.Event)
	assert.Equal(t, channel, subscriptionSucceeded.Channel)
	assert.Equal(t, data, subscriptionSucceeded.Data)
}

// ============================================================================
// ERROR DATA TESTS
// ============================================================================

func TestErrorData(t *testing.T) {
	// Test ErrorData struct
	errType := "AuthError"
	message := "Authentication failed"
	status := 401

	errorData := ErrorData{
		Type:   errType,
		Error:  message,
		Status: status,
	}

	assert.Equal(t, errType, errorData.Type)
	assert.Equal(t, message, errorData.Error)
	assert.Equal(t, status, errorData.Status)
}

func TestSubscriptionErrPack(t *testing.T) {
	// Test SubscriptionErrPack function
	channelName := constants.ChannelName("test-channel")
	errType := "AuthError"
	message := "Authentication failed"
	code := 401

	result := SubscriptionErrPack(channelName, errType, message, code)

	var subscriptionSucceeded SubscriptionSucceeded
	err := json.Unmarshal(result, &subscriptionSucceeded)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherSubscriptionError, subscriptionSucceeded.Event)
	assert.Equal(t, channelName, subscriptionSucceeded.Channel)

	// Unmarshal the data field
	var errorData ErrorData
	err = json.Unmarshal([]byte(subscriptionSucceeded.Data), &errorData)
	assert.NoError(t, err)
	assert.Equal(t, errType, errorData.Type)
	assert.Equal(t, message, errorData.Error)
	assert.Equal(t, code, errorData.Status)
}

// ============================================================================
// PRESENCE DATA TESTS
// ============================================================================

func TestPresenceData(t *testing.T) {
	// Test PresenceData struct
	ids := []string{"user1", "user2", "user3"}
	hash := map[string]map[string]string{
		"user1": {"name": "User 1"},
		"user2": {"name": "User 2"},
		"user3": {"name": "User 3"},
	}
	count := 3

	presenceData := PresenceData{
		IDs:   ids,
		Hash:  hash,
		Count: count,
	}

	assert.Equal(t, ids, presenceData.IDs)
	assert.Equal(t, hash, presenceData.Hash)
	assert.Equal(t, count, presenceData.Count)
}

func TestPresenceDataEmpty(t *testing.T) {
	// Test PresenceData with empty data
	presenceData := PresenceData{
		IDs:   []string{},
		Hash:  map[string]map[string]string{},
		Count: 0,
	}

	assert.Equal(t, 0, len(presenceData.IDs))
	assert.Equal(t, 0, len(presenceData.Hash))
	assert.Equal(t, 0, presenceData.Count)
}

// ============================================================================
// MEMBER ADDED PACK TESTS
// ============================================================================

func TestMemberAddedPack(t *testing.T) {
	// Test MemberAddedPack struct
	event := "test-event"
	data := "test-data"
	channel := constants.ChannelName("test-channel")

	memberAddedPack := MemberAddedPack{
		Event:   event,
		Data:    data,
		Channel: channel,
	}

	assert.Equal(t, event, memberAddedPack.Event)
	assert.Equal(t, data, memberAddedPack.Data)
	assert.Equal(t, channel, memberAddedPack.Channel)
}

// ============================================================================
// CLIENT CHANNEL EVENT TESTS
// ============================================================================

func TestClientChannelEvent(t *testing.T) {
	// Test ClientChannelEvent struct
	event := "client-test-event"
	data := json.RawMessage(`{"message": "Hello World"}`)
	channel := constants.ChannelName("test-channel")

	clientChannelEvent := ClientChannelEvent{
		Event:   event,
		Data:    data,
		Channel: channel,
	}

	assert.Equal(t, event, clientChannelEvent.Event)
	assert.Equal(t, data, clientChannelEvent.Data)
	assert.Equal(t, channel, clientChannelEvent.Channel)
}

func TestClientChannelEventWithStringData(t *testing.T) {
	// Test ClientChannelEvent with string data
	event := "client-test-event"
	data := json.RawMessage(`"simple string"`)
	channel := constants.ChannelName("test-channel")

	clientChannelEvent := ClientChannelEvent{
		Event:   event,
		Data:    data,
		Channel: channel,
	}

	assert.Equal(t, event, clientChannelEvent.Event)
	assert.Equal(t, data, clientChannelEvent.Data)
	assert.Equal(t, channel, clientChannelEvent.Channel)
}

// ============================================================================
// CHANNEL CACHE MISS TESTS
// ============================================================================

func TestChannelCacheMiss(t *testing.T) {
	// Test ChannelCacheMiss struct
	cacheMiss := ChannelCacheMiss{}

	// ChannelCacheMiss is an empty struct, so we just verify it can be created
	assert.NotNil(t, cacheMiss)
}

// ============================================================================
// CHANNEL EVENT TESTS
// ============================================================================

func TestChannelEvent(t *testing.T) {
	// Test ChannelEvent struct
	event := "test-event"
	data := "test-data"
	channel := constants.ChannelName("test-channel")
	userID := "user123"
	socketID := constants.SocketID("socket123")

	channelEvent := ChannelEvent{
		Event:    event,
		Data:     data,
		Channel:  channel,
		UserID:   userID,
		SocketID: socketID,
	}

	assert.Equal(t, event, channelEvent.Event)
	assert.Equal(t, data, channelEvent.Data)
	assert.Equal(t, channel, channelEvent.Channel)
	assert.Equal(t, userID, channelEvent.UserID)
	assert.Equal(t, socketID, channelEvent.SocketID)
}

func TestChannelEventMinimal(t *testing.T) {
	// Test ChannelEvent with minimal fields
	event := "test-event"

	channelEvent := ChannelEvent{
		Event: event,
	}

	assert.Equal(t, event, channelEvent.Event)
	assert.Nil(t, channelEvent.Data)
	assert.Equal(t, constants.ChannelName(""), channelEvent.Channel)
	assert.Equal(t, "", channelEvent.UserID)
	assert.Equal(t, constants.SocketID(""), channelEvent.SocketID)
}

func TestChannelEventToJson(t *testing.T) {
	// Test ChannelEvent ToJson method
	event := "test-event"
	data := map[string]interface{}{"message": "Hello World"}
	channel := constants.ChannelName("test-channel")
	userID := "user123"
	socketID := constants.SocketID("socket123")

	channelEvent := ChannelEvent{
		Event:    event,
		Data:     data,
		Channel:  channel,
		UserID:   userID,
		SocketID: socketID,
	}

	// Test with marshalDataProperty = true
	result := channelEvent.ToJson(true)

	var unmarshaled ChannelEvent
	err := json.Unmarshal(result, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, event, unmarshaled.Event)
	assert.Equal(t, channel, unmarshaled.Channel)
	assert.Equal(t, userID, unmarshaled.UserID)
	assert.Equal(t, socketID, unmarshaled.SocketID)

	// Data should be marshaled as string
	assert.IsType(t, "", unmarshaled.Data)
}

func TestChannelEventToJsonWithoutMarshal(t *testing.T) {
	// Test ChannelEvent ToJson method without marshaling data
	event := "test-event"
	data := "test-data"
	channel := constants.ChannelName("test-channel")

	channelEvent := ChannelEvent{
		Event:   event,
		Data:    data,
		Channel: channel,
	}

	// Test with marshalDataProperty = false
	result := channelEvent.ToJson(false)

	var unmarshaled ChannelEvent
	err := json.Unmarshal(result, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, event, unmarshaled.Event)
	assert.Equal(t, data, unmarshaled.Data)
	assert.Equal(t, channel, unmarshaled.Channel)
}

// ============================================================================
// CHANNEL ERROR DATA TESTS
// ============================================================================

func TestChannelErrorData(t *testing.T) {
	// Test ChannelErrorData struct
	code := 4001
	message := "Test error message"

	channelErrorData := ChannelErrorData{
		Code:    code,
		Message: message,
	}

	assert.Equal(t, code, channelErrorData.Code)
	assert.Equal(t, message, channelErrorData.Message)
}

// ============================================================================
// ERROR PACK TESTS
// ============================================================================

func TestErrorPack(t *testing.T) {
	// Test ErrorPack function
	code := util.ErrCodeInvalidPayload
	message := "Test error message"
	channel := constants.ChannelName("test-channel")

	result := ErrorPack(code, message, channel)

	var channelEvent ChannelEvent
	err := json.Unmarshal(result, &channelEvent)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, channelEvent.Event)
	assert.Equal(t, channel, channelEvent.Channel)

	// Unmarshal the data field
	var errorData ChannelErrorData

	err = json.Unmarshal([]byte(channelEvent.Data.(string)), &errorData)
	assert.NoError(t, err)
	assert.Equal(t, int(code), errorData.Code)
	assert.Equal(t, message, errorData.Message)
}

func TestErrorPackWithoutChannel(t *testing.T) {
	// Test ErrorPack function without channel
	code := util.ErrCodeInvalidPayload
	message := "Test error message"

	result := ErrorPack(code, message)

	var channelEvent ChannelEvent
	err := json.Unmarshal(result, &channelEvent)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, channelEvent.Event)
	assert.Equal(t, constants.ChannelName(""), channelEvent.Channel)

	// Unmarshal the data field
	var errorData ChannelErrorData
	err = json.Unmarshal([]byte(channelEvent.Data.(string)), &errorData)
	assert.NoError(t, err)
	assert.Equal(t, int(code), errorData.Code)
	assert.Equal(t, message, errorData.Message)
}

func TestErrorPackWithMultipleChannels(t *testing.T) {
	// Test ErrorPack function with multiple channels (should use first one)
	code := util.ErrCodeInvalidPayload
	message := "Test error message"
	channel1 := constants.ChannelName("test-channel-1")
	channel2 := constants.ChannelName("test-channel-2")

	result := ErrorPack(code, message, channel1, channel2)

	var channelEvent ChannelEvent
	err := json.Unmarshal(result, &channelEvent)
	assert.NoError(t, err)
	assert.Equal(t, constants.PusherError, channelEvent.Event)
	assert.Equal(t, channel1, channelEvent.Channel) // Should use first channel
}

// ============================================================================
// FALLBACK SERIALIZATION ERROR TESTS
// ============================================================================

func TestGetFallbackSerializationError(t *testing.T) {
	// Test getFallbackSerializationError function
	result := getFallbackSerializationError()

	// Should be valid JSON
	var payload map[string]interface{}
	err := json.Unmarshal(result, &payload)
	assert.NoError(t, err)

	// Should contain expected fields
	assert.Equal(t, constants.PusherError, payload["event"])
	assert.Contains(t, payload, "data")
}

// ============================================================================
// PUSHER API MESSAGE TESTS
// ============================================================================

func TestPusherApiMessage(t *testing.T) {
	// Test PusherApiMessage struct
	name := "test-event"
	data := "test-data"
	channel := constants.ChannelName("test-channel")
	socketID := constants.SocketID("socket123")

	pusherApiMessage := PusherApiMessage{
		Name:     name,
		Data:     data,
		Channel:  channel,
		SocketID: socketID,
	}

	assert.Equal(t, name, pusherApiMessage.Name)
	assert.Equal(t, data, pusherApiMessage.Data)
	assert.Equal(t, channel, pusherApiMessage.Channel)
	assert.Equal(t, socketID, pusherApiMessage.SocketID)
	assert.Nil(t, pusherApiMessage.Channels)
}

func TestPusherApiMessageWithChannels(t *testing.T) {
	// Test PusherApiMessage with channels array
	name := "test-event"
	data := "test-data"
	channels := []constants.ChannelName{"channel1", "channel2", "channel3"}
	socketID := constants.SocketID("socket123")

	pusherApiMessage := PusherApiMessage{
		Name:     name,
		Data:     data,
		Channels: channels,
		SocketID: socketID,
	}

	assert.Equal(t, name, pusherApiMessage.Name)
	assert.Equal(t, data, pusherApiMessage.Data)
	assert.Equal(t, constants.ChannelName(""), pusherApiMessage.Channel)
	assert.Equal(t, channels, pusherApiMessage.Channels)
	assert.Equal(t, socketID, pusherApiMessage.SocketID)
}

func TestPusherApiMessageMinimal(t *testing.T) {
	// Test PusherApiMessage with minimal fields
	name := "test-event"
	data := "test-data"

	pusherApiMessage := PusherApiMessage{
		Name: name,
		Data: data,
	}

	assert.Equal(t, name, pusherApiMessage.Name)
	assert.Equal(t, data, pusherApiMessage.Data)
	assert.Equal(t, constants.ChannelName(""), pusherApiMessage.Channel)
	assert.Nil(t, pusherApiMessage.Channels)
	assert.Equal(t, constants.SocketID(""), pusherApiMessage.SocketID)
}

// ============================================================================
// JSON MARSHALING TESTS
// ============================================================================

func TestJsonMarshaling(t *testing.T) {
	// Test that all structs can be marshaled to JSON
	structs := []interface{}{
		RequestEvent{Event: "test", Data: "data"},
		Payload{Event: "test", Data: "data"},
		EstablishData{SocketID: "socket", ActivityTimeout: 30},
		ErrData{Message: "error", Code: 4001},
		Pong{Event: "pong"},
		SubscribePayload{Event: "subscribe", Data: SubscribeChannelData{Channel: "test"}},
		SubscribeChannelData{Channel: "test"},
		UnsubscribePayload{Event: "unsubscribe", Data: UnsubscribeData{Channel: "test"}},
		UnsubscribeData{Channel: "test"},
		SubscriptionSucceeded{Event: "success", Channel: "test", Data: "data"},
		ErrorData{Type: "error", Error: "message", Status: 400},
		PresenceData{IDs: []string{"1", "2"}, Hash: map[string]map[string]string{}, Count: 2},
		MemberAddedPack{Event: "added", Data: "data", Channel: "test"},
		ClientChannelEvent{Event: "client", Data: json.RawMessage(`{}`), Channel: "test"},
		ChannelCacheMiss{},
		ChannelEvent{Event: "event", Data: "data", Channel: "test"},
		ChannelErrorData{Code: 400, Message: "error"},
		PusherApiMessage{Name: "event", Data: "data", Channel: "test"},
	}

	for i, s := range structs {
		_, err := json.Marshal(s)
		assert.NoError(t, err, "Struct %d should marshal to JSON", i)
	}
}

// ============================================================================
// EDGE CASES TESTS
// ============================================================================

func TestEmptyStrings(t *testing.T) {
	// Test with empty strings
	payload := PayloadPack("", "")

	var result Payload
	err := json.Unmarshal(payload, &result)
	assert.NoError(t, err)
	assert.Equal(t, "", result.Event)
	assert.Equal(t, "", result.Data)
}

func TestSpecialCharacters(t *testing.T) {
	// Test with special characters
	event := "test-event-with-special-chars-!@#$%^&*()"
	data := "data-with-special-chars-!@#$%^&*()"

	payload := PayloadPack(event, data)

	var result Payload
	err := json.Unmarshal(payload, &result)
	assert.NoError(t, err)
	assert.Equal(t, event, result.Event)
	assert.Equal(t, data, result.Data)
}

func TestUnicodeCharacters(t *testing.T) {
	// Test with unicode characters
	event := "test-event-with-unicode-🚀"
	data := "data-with-unicode-🚀"

	payload := PayloadPack(event, data)

	var result Payload
	err := json.Unmarshal(payload, &result)
	assert.NoError(t, err)
	assert.Equal(t, event, result.Event)
	assert.Equal(t, data, result.Data)
}
