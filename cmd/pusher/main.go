package main

import (
	"context"
	"errors"
	"flag"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pusher/api"
	"pusher/env"
	"pusher/internal"
	"pusher/internal/clients"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/dispatcher"
	"pusher/internal/middlewares"
	"pusher/internal/util"
	"pusher/internal/webhooks"
	logger "pusher/log"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Bootstrap the application by loading environment variables
	err := config.Setup()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	logger.Logger().Infoln("Booting pusher server")
	if err != nil {
		logger.Logger().Fatal(err)
	}

	fallbackPort := env.GetString("BIND_PORT", "6001")
	port := flag.String("port", fallbackPort, "port on which to run the server")

	flag.Parse()

	// Ensure that the application ID is valid; this server does not support multiple applications
	if !util.ValidAppID(env.GetString("APP_ID", "")) {
		log.Fatal(errors.New("invalid app id: " + env.GetString("APP_ID", "")))
	}

	logger.Logger().Debugln("Pusher launch in", config.AppEnv)

	// Initialize the webhook manager
	webhookDriver := env.GetString("WEBHOOK_MANAGER", "http")
	if webhookDriver == "http" {
		logger.Logger().Debugln("Webhook driver is HTTP")
		webhooks.WebhookManager = &webhooks.HttpWebhook{
			WebhookUrl: env.GetString("WEBHOOK_URL", ""),
		}
	} else if webhookDriver == "sns" {
		logger.Logger().Debugln("Webhook driver is SNS")
		sns := &webhooks.SnsWebhook{
			TopicArn: env.GetString("SNS_TOPIC_ARN", ""),
			Region:   env.GetString("SNS_REGION", ""),
		}
		snsErr := sns.InitSNS()
		if snsErr != nil {
			logger.Logger().Fatal("Failed to initialize SNS client: " + snsErr.Error())
		}
		webhooks.WebhookManager = sns
	} else {
		logger.Logger().Fatal("Invalid webhook manager driver: " + webhookDriver)
	}

	// Initialize Redis if needed
	dispatchManagerDriver := env.GetString("DISPATCHER_MANAGER", "local")
	pubSubManagerDriver := env.GetString("PUBSUB_MANAGER", "local")
	storageManagerDriver := env.GetString("STORAGE_MANAGER", "local")
	channelCacheManagerDriver := env.GetString("CHANNEL_CACHE_MANAGER", "local")

	if dispatchManagerDriver == "redis" || pubSubManagerDriver == "redis" || storageManagerDriver == "redis" || channelCacheManagerDriver == "redis" {
		logger.Logger().Debugln("Initializing Redis client")
		// Initialize Redis dispatcher here
		rc := &clients.RedisClient{}
		rErr := rc.InitRedis(env.GetString("REDIS_PREFIX", "pusher"))
		if rErr != nil {
			logger.Logger().Fatal(rErr)
		}
		clients.RedisClientInstance = rc
	}

	// Initialize the dispatcher
	if env.GetBool("WEBHOOK_ENABLED", false) {
		switch dispatchManagerDriver {
		case "local":
			logger.Logger().Debugln("Dispatcher driver is local")
			saDispatcher := &dispatcher.SyncDispatcher{}
			saDispatcher.Init()
			dispatcher.Dispatcher = saDispatcher
		case "redis":
			logger.Logger().Debugln("Dispatcher driver is redis")
			rdDispatcher := &dispatcher.RedisDispatcher{}
			rdDispatcher.Init()
			dispatcher.Dispatcher = rdDispatcher
		default:
			logger.Logger().Fatal("Invalid dispatcher manager driver: " + dispatchManagerDriver)
		}

		wg.Add(1)
		// Start the dispatcher receiver for handling new webhooks
		go func() {
			defer wg.Done()
			dispatcher.Dispatcher.ListenForEvents(ctx)
		}()
	}

	// Initialize the flap detector; this is used to throttle bounces of events
	// All events are sent to the DispatchBuffer.HandleEvent() method, which will throttle and then send to the dispatcher
	// If webhooks are disabled, all events will be dropped
	dispatcher.InitFlapDetector()

	// Create a new hub to manage client connections
	hub := internal.NewHub()
	defer handlePanic(hub)

	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run()
	}()

	// Establish the Gin router for handling HTTP requests
	if config.AppEnv == constants.PRODUCTION {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	router := gin.New()
	router.Use(middlewares.Logger(logger.Logger()))
	router.Use(gin.Recovery())

	backendGroup := router.Group("/apps", middlewares.Signature())
	{
		backendGroup.POST("/:app_id/events", api.EventTrigger)
		//backendGroup.POST("/:app_id/batch_events", api.BatchEventTrigger)
		backendGroup.GET("/:app_id/channels", api.ChannelIndex)
		backendGroup.GET("/:app_id/channels/:channel_name", api.ChannelShow)
		backendGroup.GET("/:app_id/channels/:channel_name/users", api.ChannelUsers)
	}
	router.GET("/app/:key", func(c *gin.Context) {
		appKey := c.Param("key")
		client := c.Query("client")
		version := c.Query("version")
		protocol := c.Query("protocol")

		internal.ServeWs(hub, c.Writer, c.Request, appKey, client, version, protocol)
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	server := &http.Server{
		Addr:    env.GetString("BIND_ADDR", "0.0.0.0") + ":" + *port,
		Handler: router,
	}

	// Run the web server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if webErr := server.ListenAndServe(); webErr != nil && !errors.Is(webErr, http.ErrServerClosed) {
			logger.Logger().Fatalf("Failed to start web server: %s", webErr)
		}
	}()

	handleInterrupt(hub, &wg, server, cancel)
}

func handlePanic(_ *internal.Hub) {
	if r := recover(); r != nil {
		logger.Logger().Error("Recovered from panic", r)
		//hub.HandleClosure()
		os.Exit(1)
	}
}

func handleInterrupt(hub *internal.Hub, wg *sync.WaitGroup, server *http.Server, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Logger().Infoln("Received interrupt signal, cleaning up...")
	logger.Logger().Infoln("Waiting for all tasks to complete...")

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Close the hub (which closes the stopChan)
	hub.HandleClosure()

	// Shutdown the HTTP server
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	logger.Logger().Info("... Shutting down HTTP server")
	if err := server.Shutdown(ctx); err != nil {
		logger.Logger().Errorf("HTTP server shutdown error: %v", err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Logger().Info("All tasks completed, exiting")

	os.Exit(0)
}
