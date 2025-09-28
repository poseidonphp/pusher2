package internal

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"

	"time"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

// TODO: Implement origin validation
// var upgrader = websocket.Upgrader{
// 	CheckOrigin:     func(r *http.Request) bool { return true },
// 	ReadBufferSize:  constants.MaxMessageSize,
// 	WriteBufferSize: constants.MaxMessageSize,
// }

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
		if server.Closing {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Server is not accepting connections"})
			return
		}
		appKey := c.Param("key")
		client := c.Query("client")
		version := c.Query("version")
		protocol := c.Query("protocol")

		serveWs(server, c.Writer, c.Request, appKey, client, version, protocol)
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

// serveWs handles websocket requests from the peer
// It upgrades the HTTP connection to a WebSocket, performs necessary validations,
// and initializes a WebSocket instance to manage the connection.
//
// When it's done, it adds the socket to the server's adapter and starts listening for messages.
func serveWs(server *Server, w http.ResponseWriter, r *http.Request, appKey, client, version, protocolStr string) {
	if server.Closing {
		log.Logger().Error("Server is not accepting connections")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	protocol, _ := util.Str2Int(protocolStr)

	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Logger().Errorf("error upgrading client: %s", err.Error())
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
	app, err := validateApp(server, appKey)
	if err != nil {
		closeConnectionWithError(conn, err.(util.ErrorCode), err.Error())
		return
	}

	// Check connection limits
	if err = validateConnectionLimits(server, app); err != nil {
		closeConnectionWithError(conn, err.(util.ErrorCode), err.Error())
		return
	}

	// Create and initialize WebSocket
	clientWebSocket := createWebSocket(socketID, conn, server, app)

	// Add socket to adapter
	if err = addSocketToAdapter(server, app, clientWebSocket); err != nil {
		closeConnectionWithError(conn, err.(util.ErrorCode), err.Error())
		return
	}

	// Initialize WebSocket connection
	initializeWebSocketConnection(clientWebSocket, server, app)
}

// validateApp checks if the app exists and is enabled
func validateApp(server *Server, appKey string) (*apps.App, error) {
	app, err := server.AppManager.FindByKey(appKey)
	if err != nil {
		log.Logger().Error("app not found: ", appKey)
		return nil, util.ErrCodeAppNotExist
	}

	if !app.Enabled {
		log.Logger().Error("app is disabled: ", appKey)
		return nil, util.ErrCodeAppDisabled
	}

	return app, nil
}

// validateConnectionLimits checks if the connection limits are respected
func validateConnectionLimits(server *Server, app *apps.App) error {
	socketCount := server.Adapter.GetSocketsCount(app.ID, false)

	if app.MaxConnections > -1 && socketCount+1 > app.MaxConnections {
		log.Logger().Infof("max connections exceeded for app: %s", app.ID)
		return util.ErrCodeOverQuota
	}

	return nil
}

// createWebSocket creates a new WebSocket instance
// func createWebSocket(socketID constants.SocketID, conn *websocket.Conn, server *Server, app *apps.App) *WebSocket {
func createWebSocket(socketID constants.SocketID, conn net.Conn, server *Server, app *apps.App) *WebSocket {
	if conn == nil {
		log.Logger().Error("connection is nil")
		return nil
	}

	if server == nil {
		log.Logger().Error("server is nil")
		return nil
	}

	if app == nil {
		log.Logger().Error("app is nil")
		return nil
	}

	return &WebSocket{
		ID:                 socketID,
		conn:               conn,
		closed:             false,
		server:             server,
		app:                app,
		SubscribedChannels: make(map[constants.ChannelName]*Channel),
		PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
		connReader:         wsutil.NewReader(conn, ws.StateServerSide),
		connWriter:         wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText),
	}
}

// addSocketToAdapter adds the socket to the server's adapter
func addSocketToAdapter(server *Server, app *apps.App, ws *WebSocket) error {
	err := server.Adapter.AddSocket(app.ID, ws)
	if err != nil {
		log.Logger().Errorf("failed to add socket: %s, %s ", ws.ID, err.Error())
		return util.ErrCodeOverQuota
	}
	return nil
}

// initializeWebSocketConnection initializes the WebSocket connection
func initializeWebSocketConnection(ws *WebSocket, server *Server, app *apps.App) {
	// broadcast the pusher:connection_established
	go ws.Listen()
	ws.Send(payloads.EstablishPack(ws.ID, app.ActivityTimeout))

	if app.RequireChannelAuthorization {
		// if require_channel_authorization is set to true for the app, set a timeout for the authorization to happen
		// This closes any conns that don't subscribe to a private/presence within x seconds
		ws.WatchForAuthentication()
	}

	// Mark new connection in metrics
	server.MetricsManager.MarkNewConnection(app.ID)
}

// closeConnectionWithError closes the connection with an error message
func closeConnectionWithError(conn net.Conn, errorCode util.ErrorCode, message string) {
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = wsutil.WriteServerMessage(conn, ws.OpText, payloads.ErrorPack(errorCode, message))
	_ = conn.Close()
}
