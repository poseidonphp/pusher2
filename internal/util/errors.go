package util

import (
	"errors"
)

type ErrorCode int

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

const (
	// error codes 4000-4099 = Close connection; reconnecting using same params will fail
	ErrCodeSSLRequired                ErrorCode = 4000
	ErrCodeAppNotExist                ErrorCode = 4001
	ErrCodeAppDisabled                ErrorCode = 4003
	ErrCodeOverQuota                  ErrorCode = 4004
	ErrCodePathNotFound               ErrorCode = 4005
	ErrCodeInvalidVersionString       ErrorCode = 4006
	ErrCodeUnsupportedProtocolVersion ErrorCode = 4007
	ErrCodeNoProtocolVersion          ErrorCode = 4008
	ErrCodeUnauthorizedConnection     ErrorCode = 4009
	ErroCodeInternalServerError       ErrorCode = 4050

	// Error codes 4100-4199 = Try again after 1s
	ErrCodeOverCapacity       ErrorCode = 4100
	ErrCodeServerShuttingDown ErrorCode = 4105

	// Error codes 4200-4299 = Closed by backend; Generic reconnect immediately
	ErrCodeGenericReconnect      ErrorCode = 4200
	ErrCodePongNotReceived       ErrorCode = 4201
	ErrCodeClosedAfterInactivity ErrorCode = 4202
	ErrCodeCloseExpected         ErrorCode = 4203

	// Error codes 4300-4399 = Client event rejected; generic error
	ErrCodeClientEventRejected               ErrorCode = 4301
	ErrCodeInvalidPayload                    ErrorCode = 4310
	ErrCodeInvalidChannel                    ErrorCode = 4311
	ErrCodeAlreadySubscribed                 ErrorCode = 4312
	ErrCodeMaxPresenceSubscribers            ErrorCode = 4313
	ErrCodePresenceUserDataTooMuch           ErrorCode = 4314
	ErrCodePresenceUserIDTooLong             ErrorCode = 4315
	ErrCodeNotSubscribed                     ErrorCode = 4316
	ErrCodeClientOnlySupportsPrivatePresence ErrorCode = 4317
	ErrCodeSubscriptionAccessDenied          ErrorCode = 4318

	ErrCodeWebsocketAbnormalClosure ErrorCode = 4400
	ErrCodeGoRoutineExited          ErrorCode = 4401
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
		ErrCodeMaxPresenceSubscribers:  "presence channel user limit reached",
		ErrCodePresenceUserDataTooMuch: "presence channel user data exceeds limit (MAX_PRESENCE_USER_DATA_KB)",
		ErrCodePresenceUserIDTooLong:   "presence channel limit to 128 characters user id",
		ErrCodeNotSubscribed:           "Not subscribed",

		ErroCodeInternalServerError: "Internal server error",

		ErrCodeClientOnlySupportsPrivatePresence: "Client only supports private and presence channels",
		ErrCodeSubscriptionAccessDenied:          "Subscription access denied",

		ErrCodeServerShuttingDown: "Server shutting down",

		ErrCodeGoRoutineExited:          "Go routine exited",
		ErrCodeOverCapacity:             "Over capacity",
		ErrCodeGenericReconnect:         "Generic reconnect immediately",
		ErrCodePongNotReceived:          "Pong reply not received: ping was sent to the client, but no reply was received - see ping and pong messages",
		ErrCodeClosedAfterInactivity:    "Closed after inactivity: Client has been inactive for a long time (currently 24 hours) and client does not support ping. Please upgrade to a newer WebSocket draft or implement version 5 or above of this protocol",
		ErrCodeCloseExpected:            "Close expected",
		ErrCodeClientEventRejected:      "Client event rejected due to rate limit",
		ErrCodeWebsocketAbnormalClosure: "Websocket abnormal closure",
	}

	ErrInvalidChannel                 = errors.New("invalid channel name")
	ErrAPIReqEventChannelsSizeTooLong = errors.New("request event channels size too big, limit to 100")

	// ErrAPIReqEventDataTooLarge        = fmt.Errorf("request event data too large, limit to %dkb", constants.MaxEventSizeKb)
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
