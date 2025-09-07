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
	"pusher/internal/metrics"

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

	// Bootstrap the application by reading command line flags, environment variables, and config files
	// to build the server configuration.
	ctx, cancel := context.WithCancel(context.Background())
	serverConfig, err := config.InitializeServerConfig(&ctx)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Logger().Debugf("Loaded config: %+v\n", serverConfig)

	logger.Logger().Infoln("Booting pusher server")

	// Create the server instance
	var server *internal.Server
	server, err = internal.NewServer(ctx, serverConfig)
	if err != nil {
		logger.Logger().Fatal("Failed to create server: " + err.Error())
		return
	}

	if server == nil {
		logger.Logger().Fatal("Server instance is nil")
		return
	}

	// Ensure we handle panics gracefully when the main function exits
	defer handlePanic(server)

	// Load and start the web server to handle incoming HTTP requests
	webServer := internal.LoadWebServer(server)
	webServer.RegisterOnShutdown(func() {
		logger.Logger().Infoln("Registered Shutdown handler: Web server is shutting down...")
		server.Closing = true
		server.CloseAllLocalSockets()
	})

	// Run the web server in a goroutine with a wait group to manage its lifecycle
	// and ensure it can be gracefully shut down on interrupt signals.
	// The web server will listen on the configured port and bind address.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if webErr := webServer.ListenAndServe(); webErr != nil && !errors.Is(webErr, http.ErrServerClosed) {
			logger.Logger().Fatalf("Failed to start web server: %s", webErr)
		}
		logger.Logger().Infoln("Web server stopped")
	}()

	// If metrics are enabled in the configuration, start the metrics server
	// in a separate goroutine. This server will expose Prometheus metrics
	// on the specified metrics port.
	// It will also be gracefully shut down on interrupt signals.
	// The metrics server is optional and only started if enabled.
	// It uses the same wait group to manage its lifecycle.
	var metricsServer *http.Server
	if serverConfig.MetricsEnabled && server.MetricsManager != nil {
		metricsServer = serveMetrics(server, serverConfig, &wg)
	}

	// infinitely wait for interrupt signals to gracefully shut down the server and clean up resources.
	handleInterrupt(server, &wg, webServer, metricsServer, cancel)
}

func serveMetrics(server *internal.Server, serverConfig *config.ServerConfig, wg *sync.WaitGroup) *http.Server {
	mux := http.NewServeMux()
	mh := metrics.NewMetricsHandler(server.PrometheusMetrics)
	mux.Handle("/metrics", mh.TextHandler())
	addr := fmt.Sprintf("%s:%s", serverConfig.BindAddress, serverConfig.MetricsPort)
	metricsServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Logger().Infof("Starting metrics server on %s:%s/metrics\n", serverConfig.BindAddress, serverConfig.MetricsPort)
		if metricsErr := metricsServer.ListenAndServe(); metricsErr != nil && !errors.Is(metricsErr, http.ErrServerClosed) {
			logger.Logger().Fatalf("Failed to start metrics server: %s", metricsErr)
		}
		logger.Logger().Infoln("Metrics server stopped")
	}()
	return metricsServer
}

// handlePanic recovers from panics, logs the error, attempts to close all local sockets,
// and exits the application.
func handlePanic(server *internal.Server) {
	if r := recover(); r != nil {
		logger.Logger().Error("Recovered from panic", r)
		server.Closing = true
		server.CloseAllLocalSockets()
		os.Exit(1)
	}
}

// handleInterrupt listens for OS interrupt signals (like Ctrl+C) and initiates a graceful shutdown
func handleInterrupt(server *internal.Server, wg *sync.WaitGroup, webServer *http.Server, metricsServer *http.Server, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Logger().Infoln("Received interrupt signal, waiting for all tasks to complete...")

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Shutdown the HTTP server
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	logger.Logger().Info("... Shutting down HTTP server")
	if err := webServer.Shutdown(ctx); err != nil {
		logger.Logger().Errorf("HTTP server shutdown error: %v", err)
	}

	if metricsServer != nil {
		ctxMetrics, shutdownCancelMetrics := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancelMetrics()

		logger.Logger().Info("... Shutting down Metrics server")
		if err := metricsServer.Shutdown(ctxMetrics); err != nil {
			logger.Logger().Errorf("Metrics server shutdown error: %v", err)
		}
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Logger().Info("All tasks completed, exiting")

	os.Exit(0)
}
