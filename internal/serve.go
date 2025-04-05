package internal

import (
	"github.com/gorilla/websocket"
	"github.com/thoas/go-funk"
	"net/http"
	"pusher/env"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
	"time"
)

// ServeWS handles websocket requests from the peer
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, appKey, client, version, protocolStr string) {
	protocol, _ := util.Str2Int(protocolStr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Logger().Error(err)
		return
	}

	if appKey != env.GetString("APP_KEY", "") {
		log.Logger().Error("Error app key", appKey)
		_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, payloads.ErrPack(util.ErrCodeAppNotExist))
		_ = conn.Close()
		return
	}

	if !funk.Contains(constants.SupportedProtocolVersions, protocol) {
		log.Logger().Error("Unsupported protocol version", protocol)
		_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, payloads.ErrPack(util.ErrCodeUnsupportedProtocolVersion))
		_ = conn.Close()
		return
	}
	socketID := util.GenerateSocketID()
	log.Logger().Tracef("New socket id: %s\n", socketID)
	session := &Session{
		conn:             conn,
		client:           client,
		version:          version,
		protocol:         protocol,
		sendChannel:      make(chan []byte, constants.MaxMessageSize),
		subscriptions:    make(map[constants.ChannelName]bool),
		socketID:         socketID,
		closed:           false,
		done:             make(chan struct{}),
		presenceChannels: make(map[constants.ChannelName]constants.ChannelName),
	}

	hub.register <- session
	go session.senderSubProcess()
	go session.readerSubProcess()

	// Send message to client confirming the established connection
	session.Send(payloads.EstablishPack(socketID))
}

// TODO: Implement origin validation
var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  constants.MaxMessageSize,
	WriteBufferSize: constants.MaxMessageSize,
}
