package internal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/thoas/go-funk"
	"pusher/internal/config"

	"time"

	"pusher/internal/constants"
	"pusher/internal/middlewares"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

// TODO: Implement origin validation
var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  constants.MaxMessageSize,
	WriteBufferSize: constants.MaxMessageSize,
}

func LoadWebServer(serverConfig *config.ServerConfig, hub *Hub) *http.Server {
	// Establish the Gin router for handling HTTP requests
	if serverConfig.Env == constants.PRODUCTION {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	router := gin.New()
	router.Use(middlewares.Logger(log.Logger()))
	router.Use(gin.Recovery())

	backendGroup := router.Group("/apps", middlewares.Signature(serverConfig))
	{
		backendGroup.POST("/:app_id/events", func(c *gin.Context) {
			EventTrigger(c, hub)
		})

		backendGroup.POST("/:app_id/batch_events", func(c *gin.Context) {
			BatchEventTrigger(c, hub)
		})

		backendGroup.GET("/:app_id/channels", func(c *gin.Context) {
			ChannelIndex(c, serverConfig)
		})

		backendGroup.GET("/:app_id/channels/:channel_name", func(c *gin.Context) {
			ChannelShow(c, serverConfig)
		})

		backendGroup.GET("/:app_id/channels/:channel_name/users", func(c *gin.Context) {
			ChannelUsers(c, serverConfig)
		})

	}
	router.GET("/app/:key", func(c *gin.Context) {
		appKey := c.Param("key")
		client := c.Query("client")
		version := c.Query("version")
		protocol := c.Query("protocol")

		ServeWs(hub, c.Writer, c.Request, appKey, client, version, protocol)
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	server := &http.Server{
		Addr:    serverConfig.BindAddress + ":" + serverConfig.Port,
		Handler: router,
	}
	return server
}

// ServeWS handles websocket requests from the peer
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, appKey, client, version, protocolStr string) {
	if !hub.IsAcceptingConnections() {
		log.Logger().Error("Server is not accepting connections")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	protocol, _ := util.Str2Int(protocolStr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Logger().Error(err)
		return
	}

	_, appErr := hub.config.LoadAppByKey(appKey)
	if appErr != nil {
		log.Logger().Error("Error app key: ", appKey)
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
		hub:              hub,
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
	go session.senderSubProcess(hub.ctx)
	go session.readerSubProcess()

	// Send message to client confirming the established connection
	session.Send(payloads.EstablishPack(socketID))
}
