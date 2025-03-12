package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"os"
	"os/signal"
	"pusher/api"
	"pusher/env"
	"pusher/internal"
	"pusher/internal/config"
	"pusher/internal/middlewares"
	"pusher/internal/util"
	logger "pusher/log"
	"runtime"
	"syscall"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Bootstrap the application by loading environment variables
	err := config.Setup()
	logger.Logger().Infoln("Booting pusher server")
	if err != nil {
		logger.Logger().Fatal(err)
	}
	fallbackPort := env.GetString("BIND_PORT", "6001")
	port := flag.String("port", fallbackPort, "port to run the server on")
	flag.Parse()

	// Ensure that the application ID is valid; this server does not support multiple applications
	if !util.ValidAppID(env.GetString("APP_ID", "")) {
		log.Fatal(errors.New("invalid app id: " + env.GetString("APP_ID", "")))
	}

	logger.Logger().Traceln("Pusher launch in", config.AppEnv)

	// Create a new hub to manage client connections
	hub := internal.NewHub()
	defer handlePanic(hub)
	logger.Logger().Traceln("Starting pusher server")
	go hub.Run()
	// Establish the Gin router for handling HTTP requests
	router := gin.New()
	router.Use(middlewares.Logger(logger.Logger()))
	router.Use(gin.Recovery())

	backendGroup := router.Group("/apps", middlewares.Signature())
	{
		backendGroup.POST("/:app_id/events", api.EventTrigger)
		//backendGroup.POST("/:app_id/batch_events", api.BatchEventTrigger)
		//backendGroup.GET("/:app_id/channels", api.ChannelIndex)
		//backendGroup.GET("/:app_id/channels/:channel_name", api.ChannelShow)
		//backendGroup.GET("/:app_id/channels/:channel_name/users", api.ChannelUsers)
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

	// Run the web server in a goroutine
	go func() {
		_ = router.Run(env.GetString("BIND_ADDR", "0.0.0.0") + ":" + *port)
	}()

	handleInterrupt(hub)
}

func handlePanic(_ *internal.Hub) {
	if r := recover(); r != nil {
		logger.Logger().Error("Recovered from panic", r)
		//hub.HandleClosure()
		os.Exit(1)
	}
}

func handleInterrupt(_ *internal.Hub) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Closing...")
	logger.Logger().Warn("Received interrupt signal, cleaning up...")
	//hub.HandleClosure()
	os.Exit(0)
}

//TODO: implement webhook validation (pusher library: client.go -> func (c *Client) Webhook)
