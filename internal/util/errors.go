package util

import (
	"errors"
	"fmt"
	"pusher/internal/config"
	"pusher/internal/constants"
)

type ErrorCode int

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

const (
	ErrCodeSSLRequired                ErrorCode = 4000
	ErrCodeAppNotExist                ErrorCode = 4001
	ErrCodeAppDisabled                ErrorCode = 4003
	ErrCodeOverQuota                  ErrorCode = 4004
	ErrCodePathNotFound               ErrorCode = 4005
	ErrCodeInvalidVersionString       ErrorCode = 4006
	ErrCodeUnsupportedProtocolVersion ErrorCode = 4007
	ErrCodeNoProtocolVersion          ErrorCode = 4008
	ErrCodeUnauthorizedConnection     ErrorCode = 4009

	ErrCodeInvalidPayload                    ErrorCode = 4010
	ErrCodeInvalidChannel                    ErrorCode = 4011
	ErrCodeAlreadySubscribed                 ErrorCode = 4012
	ErrCodeMaxPresenceSubscribers            ErrorCode = 4013
	ErrCodePresenceUserDataTooMuch           ErrorCode = 4014
	ErrCodePresenceUserIDTooLong             ErrorCode = 4015
	ErrCodeNotSubscribed                     ErrorCode = 4016
	ErrCodeClientOnlySupportsPrivatePresence ErrorCode = 4017

	ErrCodeOverCapacity          ErrorCode = 4100
	ErrCodeGenericReconnect      ErrorCode = 4200
	ErrCodePongNotReceived       ErrorCode = 4201
	ErrCodeClosedAfterInactivity ErrorCode = 4202
	ErrCodeCloseExpected         ErrorCode = 4203

	ErrCodeClientEventRejected      ErrorCode = 4301
	ErrCodeWebsocketAbnormalClosure ErrorCode = 4400
	ErrCodeGoRoutineExited          ErrorCode = 4401

	// error codes 4000-4099 = Close connection; reconnecting using same params will fail
	// Error codes 4100-4199 = Try again after 1s
	// Error codes 4200-4299 = Generic reconnect immediately
)

// @doc https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol#error-codes
var (
	ErrCodes = map[ErrorCode]string{
		ErrCodeSSLRequired:                "Application only accepts SSL connections, reconnect using wss://",
		ErrCodeAppNotExist:                "Application does not exist",
		ErrCodeAppDisabled:                "Application disabled",
		ErrCodeOverQuota:                  "Application is over connection quota",
		ErrCodePathNotFound:               "Path not found",
		ErrCodeInvalidVersionString:       "Invalid version string format",
		ErrCodeUnsupportedProtocolVersion: "Unsupported protocol version",
		ErrCodeNoProtocolVersion:          "No protocol version supplied",
		ErrCodeUnauthorizedConnection:     "Connection is unauthorized",

		ErrCodeInvalidPayload:          "Invalid payload",
		ErrCodeInvalidChannel:          "Invalid channel",
		ErrCodeAlreadySubscribed:       "Already subscribed",
		ErrCodeMaxPresenceSubscribers:  fmt.Sprintf("presence channel limit to %d members maximum", config.MaxPresenceUsers),
		ErrCodePresenceUserDataTooMuch: "presence channel limit to 1KB user object",
		ErrCodePresenceUserIDTooLong:   "presence channel limit to 128 characters user id",
		ErrCodeNotSubscribed:           "Not subscribed",

		ErrCodeClientOnlySupportsPrivatePresence: "Client only supports private and presence channels",

		ErrCodeGoRoutineExited:          "Go routine exited",
		ErrCodeOverCapacity:             "Over capacity",
		ErrCodeGenericReconnect:         "Generic reconnect immediately",
		ErrCodePongNotReceived:          "Pong reply not received: ping was sent to the client, but no reply was received - see ping and pong messages",
		ErrCodeClosedAfterInactivity:    "Closed after inactivity: Client has been inactive for a long time (currently 24 hours) and client does not support ping. Please upgrade to a newer WebSocket draft or implement version 5 or above of this protocol",
		ErrCodeCloseExpected:            "Close expected",
		ErrCodeClientEventRejected:      "Client event rejected due to rate limit",
		ErrCodeWebsocketAbnormalClosure: "Websocket abnormal closure",
	}

	ErrInvalidChannel                 = errors.New("channel's name are invalid")
	ErrAPIReqEventDataTooLarge        = fmt.Errorf("request event data too large, limit to %dkb", constants.MaxEventSizeKb)
	ErrAPIReqEventChannelsSizeTooLong = errors.New("request event channels size too big, limit to 100")
)

func (e ErrorCode) Error() string {
	if msg, ok := ErrCodes[e]; ok {
		return msg
	}
	return "Unknown error"
}

func NewError(code ErrorCode, appendToMessage ...string) *Error {
	e := &Error{
		Code:    code,
		Message: code.Error(),
	}
	for _, appendMsg := range appendToMessage {
		e.Message += " : " + appendMsg
	}
	return e
}

func NewUnknownError(message string) *Error {
	return &Error{
		Code:    9999,
		Message: message,
	}
}
