package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"net/http"
	"os"
	"os/signal"

	"runtime"
	"sync"
	"syscall"
	"time"

	"pusher/internal"

	// _ "net/http/pprof"

	"pusher/internal/config"
	logger "pusher/log"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/**** PROFILING - REMOVE BEFORE PRODUCTION ****/
	// also imported _ "net/http/pprof"
	// go func() {
	// 	log.Logger().Println("Starting pprof on :6060")
	// 	log.Logger().Println(http.ListenAndServe("localhost:6060", nil))
	// }()
	/**** END PROFILING ****/
}

func main() {
	// Bootstrap the application by loading environment variables
	ctx, cancel := context.WithCancel(context.Background())
	serverConfig, err := config.InitializeServerConfigNew(&ctx)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Printf("Loaded config: %+v\n", serverConfig)

	// commandLineFlags := config.ParseCommandLineFlags()
	//
	// serverConfig, err := config.InitializeServerConfig(&ctx, commandLineFlags)
	// if err != nil {
	// 	logger.Logger().Fatal(err)
	// }

	var wg sync.WaitGroup

	logger.Logger().Infoln("Booting pusher server")

	// server, err := internal.NewServer(ctx, serverConfig, commandLineFlags)
	server, err := internal.NewServer(ctx, serverConfig)
	if err != nil {
		logger.Logger().Fatal("Failed to create server: " + err.Error())
	}

	defer handlePanic(server)

	webServer := internal.LoadWebServer(server)

	// Run the web server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if webErr := webServer.ListenAndServe(); webErr != nil && !errors.Is(webErr, http.ErrServerClosed) {
			logger.Logger().Fatalf("Failed to start web server: %s", webErr)
		}
	}()

	handleInterrupt(server, &wg, webServer, cancel)
}

func handlePanic(server *internal.Server) {
	if r := recover(); r != nil {
		logger.Logger().Error("Recovered from panic", r)
		server.Closing = true
		server.CloseAllLocalSockets()
		os.Exit(1)
	}
}

func handleInterrupt(server *internal.Server, wg *sync.WaitGroup, webServer *http.Server, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Logger().Infoln("Received interrupt signal, waiting for all tasks to complete...")

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Close the server
	server.Closing = true
	server.CloseAllLocalSockets()

	// Shutdown the HTTP server
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	logger.Logger().Info("... Shutting down HTTP server")
	if err := webServer.Shutdown(ctx); err != nil {
		logger.Logger().Errorf("HTTP server shutdown error: %v", err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Logger().Info("All tasks completed, exiting")

	os.Exit(0)
}
