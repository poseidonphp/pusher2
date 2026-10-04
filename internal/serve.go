package internal

import (
	"net"
	"net/http"
	"slices"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"

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
	webServer := &http.Server{
		Addr:    server.config.BindAddress + ":" + server.config.Port,
		Handler: chain(newRouter(server), LoggerMiddleware(log.Logger()), RecoveryMiddleware),
	}
	return webServer
}

// newRouter registers the websocket, REST API and health routes.
func newRouter(server *Server) *http.ServeMux {
	mux := http.NewServeMux()

	backend := func(handle func(http.ResponseWriter, *http.Request, *Server)) http.Handler {
		return chain(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handle(w, r, server) }),
			CorsMiddleware(server),
			AppMiddleware(server), // this should come early as it will set the app in the context
			SignatureMiddleware(server),
			RateLimiterMiddleware(server),
		)
	}

	mux.Handle("POST /apps/{app_id}/events", backend(EventTrigger))
	mux.Handle("POST /apps/{app_id}/batch_events", backend(BatchEventTrigger))
	mux.Handle("GET /apps/{app_id}/channels", backend(ChannelIndex))
	mux.Handle("GET /apps/{app_id}/channels/{channel_name}", backend(ChannelShow))
	mux.Handle("GET /apps/{app_id}/channels/{channel_name}/users", backend(ChannelUsers))
	mux.Handle("POST /apps/{app_id}/users/{user_id}/terminate_connections", backend(TerminateUserConnections))

	mux.HandleFunc("GET /app/{key}", func(w http.ResponseWriter, r *http.Request) {
		if server.Closing {
			writeError(w, http.StatusServiceUnavailable, "Server is not accepting connections")
			return
		}
		query := r.URL.Query()
		serveWs(server, w, r, r.PathValue("key"), query.Get("client"), query.Get("version"), query.Get("protocol"))
	})

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
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

	if !slices.Contains(constants.SupportedProtocolVersions, protocol) {
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
