package main

import (
	"context"
	"errors"

	"net/http"
	"os"
	"os/signal"

	"pusher/internal"

	"runtime"
	"sync"
	"syscall"
	"time"

	"pusher/internal/config"
	logger "pusher/log"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Bootstrap the application by loading environment variables
	ctx, cancel := context.WithCancel(context.Background())

	commandLineFlags := config.ParseCommandLineFlags()

	serverConfig, err := config.InitializeServerConfig(&ctx, commandLineFlags)
	if err != nil {
		logger.Logger().Fatal(err)
	}

	var wg sync.WaitGroup

	logger.Logger().Infoln("Booting pusher server")

	err = serverConfig.InitializeBackendServices()
	if err != nil {
		logger.Logger().Fatal("Failed to initialize backend services: " + err.Error())
	}

	// Create a new hub to manage client connections
	hub := internal.NewHub(ctx, serverConfig)
	defer handlePanic(hub)

	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run()
	}()

	server := internal.LoadWebServer(serverConfig, hub)

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
		// hub.HandleClosure()
		os.Exit(1)
	}
}

func handleInterrupt(hub *internal.Hub, wg *sync.WaitGroup, server *http.Server, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Logger().Infoln("Received interrupt signal, waiting for all tasks to complete...")

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
