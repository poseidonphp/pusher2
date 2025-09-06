package internal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"

	"time"

	"pusher/internal/constants"
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

// LoadWebServer initializes and returns an HTTP server configured with routes and middleware
// for handling WebSocket connections and REST API requests.
//
// It does not start the server; it only sets it up, then returns it to the caller.
func LoadWebServer(server *Server) *http.Server {
	// Establish the Gin router for handling HTTP requests
	if server.config.Env == constants.PRODUCTION {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	router := gin.New()
	router.Use(LoggerMiddleware(log.Logger()))
	router.Use(gin.Recovery())

	backendGroup := router.Group("/apps",
		CorsMiddleware(server),
		AppMiddleware(server), // this should come early as it will set the app in the context
		SignatureMiddleware(server),
		RateLimiterMiddleware(server),
	)
	{
		backendGroup.POST("/:app_id/events", func(c *gin.Context) {
			EventTrigger(c, server)
		})

		backendGroup.POST("/:app_id/batch_events", func(c *gin.Context) {
			BatchEventTrigger(c, server)
		})

		backendGroup.GET("/:app_id/channels", func(c *gin.Context) {
			ChannelIndex(c, server)
		})

		backendGroup.GET("/:app_id/channels/:channel_name", func(c *gin.Context) {
			ChannelShow(c, server)
		})

		backendGroup.GET("/:app_id/channels/:channel_name/users", func(c *gin.Context) {
			ChannelUsers(c, server)
		})

		//    POST /apps/[app_id]/users/[user_id]/terminate_connections
		backendGroup.POST("/:app_id/users/:user_id/terminate_connections", func(c *gin.Context) {
			TerminateUserConnections(c, server)
		})

	}
	router.GET("/app/:key", func(c *gin.Context) {
		appKey := c.Param("key")
		client := c.Query("client")
		version := c.Query("version")
		protocol := c.Query("protocol")

		ServeWs(server, c.Writer, c.Request, appKey, client, version, protocol)
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	webServer := &http.Server{
		Addr:    server.config.BindAddress + ":" + server.config.Port,
		Handler: router,
	}
	return webServer
}

// ServeWs handles websocket requests from the peer
// It upgrades the HTTP connection to a WebSocket, performs necessary validations,
// and initializes a WebSocket instance to manage the connection.
//
// When it's done, it adds the socket to the server's adapter and starts listening for messages.
func ServeWs(server *Server, w http.ResponseWriter, r *http.Request, appKey, client, version, protocolStr string) {
	if server.Closing {
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

	if !funk.Contains(constants.SupportedProtocolVersions, protocol) {
		log.Logger().Debug("Unsupported protocol version", protocol)
		closeConnectionWithError(conn, util.ErrCodeUnsupportedProtocolVersion, "unsupported protocol version")
		return
	}
	socketID := util.GenerateSocketID()
	log.Logger().Tracef("New socket id: %s\n", socketID)

	// Check for valid app
	app, err := server.AppManager.FindByKey(appKey)
	if err != nil {
		log.Logger().Error("app not found: ", appKey)
		closeConnectionWithError(conn, util.ErrCodeAppNotExist, "app not found")
		return
	}

	if !app.Enabled {
		log.Logger().Error("app is disabled: ", appKey)
		closeConnectionWithError(conn, util.ErrCodeAppDisabled, "app is disabled")
		return
	}

	ws := &WebSocket{
		ID:   socketID,
		conn: conn,
		// done:               make(chan struct{}),
		closed:             false,
		server:             server,
		app:                app,
		SubscribedChannels: make(map[constants.ChannelName]*Channel),
		PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
	}

	// perform app specific checks like max connections, app is enabled, etc.

	socketCount := server.Adapter.GetSocketsCount(app.ID, false)

	if app.MaxConnections > -1 && socketCount+1 > app.MaxConnections {
		log.Logger().Infof("max connections exceeded for app: %s", app.ID)
		closeConnectionWithError(conn, util.ErrCodeOverQuota, "maximum number of connections has been reached")
		return
	}

	err = server.Adapter.AddSocket(app.ID, ws)
	if err != nil {
		log.Logger().Errorf("failed to add socket: %s, %s ", ws.ID, err.Error())
		closeConnectionWithError(conn, util.ErrCodeOverQuota, "internal error while adding socket")
		return
	}

	// broadcast the pusher:connection_established
	go ws.Listen()
	ws.Send(payloads.EstablishPack(socketID, app.ActivityTimeout))

	if app.RequireChannelAuthorization {
		// if require_channel_authorization is set to true for the app, set a timeout for the authorization to happen
		// This closes any conns that don't subscribe to a private/presence within x seconds
		ws.WatchForAuthentication()
	}

	// Mark new connection in metrics
	server.MetricsManager.MarkNewConnection(app.ID)
}

// closeConnectionWithError closes the connection with an error message
func closeConnectionWithError(conn *websocket.Conn, errorCode util.ErrorCode, message string) {
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = conn.WriteMessage(websocket.TextMessage, payloads.ErrorPack(errorCode, message))
	_ = conn.Close()
}
