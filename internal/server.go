package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pusher/internal/apps"
	"pusher/internal/cache"
	"pusher/internal/config"
	"pusher/internal/metrics"
	"pusher/internal/queues"
	"pusher/internal/webhooks"
	"pusher/log"
)

type Server struct {
	config            *config.ServerConfig
	ctx               context.Context
	AppManager        apps.AppManagerInterface
	Adapter           AdapterInterface
	MetricsManager    metrics.MetricsInterface
	PrometheusMetrics *metrics.PrometheusMetrics
	CacheManager      cache.CacheContract
	QueueManager      queues.QueueInterface
	WebhookSender     *webhooks.WebhookSender
	Closing           bool
	// websocketPool  sync.Pool
}

// NewServer creates a new server instance with the provided configuration.
//
// This does not run the web server, but instead manages the storage of connected clients
// and handles the pub/sub system.
//
// It initializes various components such as the adapter, app manager, metrics manager,
// cache manager, and queue manager based on the provided configuration.
func NewServer(ctx context.Context, conf *config.ServerConfig) (*Server, error) {
	if conf == nil {
		return nil, errors.New("server config is nil")
	}

	if conf.Applications == nil {
		return nil, errors.New("ServerConfig.Applications cannot be nil")
	}

	s := &Server{
		ctx:    ctx,
		config: conf,
	}

	// Initialize metrics manager first
	var metricsManager metrics.MetricsInterface
	if conf.MetricsEnabled {
		metricsLabels := make(map[string]string, 0)
		pm := metrics.NewPrometheusMetricsWithLabels("socketrush", "mac", metricsLabels)
		s.PrometheusMetrics = pm
		metricsManager = pm
	} else {
		metricsManager = &metrics.NoOpMetrics{}
	}

	adapter, err := loadAdapter(ctx, conf, metricsManager)
	if err != nil {
		return nil, err
	}

	appManager, err := loadAppManager(ctx, conf)
	if err != nil {
		return nil, err
	}

	cacheManager, err := loadCacheManager(ctx, conf)
	if err != nil {
		return nil, err
	}

	// var webhookManager webhooks.WebhookInterface
	// if conf.WebhookEnabled {
	// 	switch conf.WebhookDriver {
	// 	case "http":
	// 		webhookManager = &webhooks.HttpWebhook{WebhookUrl: conf.WebhookURL}
	// 	case "sns":
	// 		webhookManager = &webhooks.SnsWebhook{TopicArn: conf.WebhookSnsTopicArn, Region: conf.WebhookSnsRegion}
	// 	}
	// }

	awsRegion := os.Getenv("AWS_REGION")

	snsHook, snsErr := webhooks.NewSnsWebhook(awsRegion)
	if snsErr != nil {
		return nil, snsErr
	}

	webhookSender := &webhooks.WebhookSender{
		HttpSender: &webhooks.HttpWebhook{},
		SNSSender:  snsHook,
	}

	queueManager, err := loadQueueManager(ctx, conf, webhookSender)
	if err != nil {
		return nil, err
	}

	s.AppManager = appManager
	s.Adapter = adapter
	s.MetricsManager = metricsManager
	s.CacheManager = cacheManager
	s.QueueManager = queueManager
	s.WebhookSender = webhookSender

	// s := &Server{
	// 	ctx:            ctx,
	// 	config:         conf,
	// 	AppManager:     appManager,
	// 	Adapter:        adapter,
	// 	MetricsManager: metricsManager,
	// 	CacheManager:   cacheManager,
	// 	QueueManager:   queueManager,
	// 	WebhookSender:  webhookSender,
	// }

	return s, nil
}

// Run starts the server and web server, handling configuration and errors.
func Run(parentCtx context.Context) error {
	if parentCtx == nil {
		return errors.New("parent context is nil")
	}
	ctx, cancel := context.WithCancel(parentCtx)
	// Bootstrap the application by reading command line flags, environment variables, and config files
	// to build the server configuration.
	serverConfig, err := config.InitializeServerConfig(&ctx)
	if err != nil {
		cancel()
		return errors.New("failed to load config: " + err.Error())
	}

	log.Logger().Infoln("Booting pusher server")

	// Create the server instance
	server, serverErr := NewServer(ctx, serverConfig)
	if serverErr != nil {
		cancel()
		return errors.New("failed to create server: " + serverErr.Error())
	}
	if server == nil {
		cancel()
		return fmt.Errorf("server instance is nil")
	}

	// Ensure we handle panics gracefully when the main function exits
	defer handlePanic(server)

	// Load and start the web server to handle incoming HTTP requests
	webServer := LoadWebServer(server)
	if webServer == nil {
		cancel()
		return fmt.Errorf("web server instance is nil")
	}

	serverErrChan := make(chan error, 1)
	sigChan := make(chan os.Signal, 1)
	// Start the web server in a new goroutine
	go func() {
		if startServerErr := webServer.ListenAndServe(); startServerErr != nil && !errors.Is(http.ErrServerClosed, err) {
			serverErrChan <- fmt.Errorf("failed to start web server: %s", startServerErr.Error())
		}
	}()

	// If metrics are enabled in the configuration, start the metrics server
	// in a separate goroutine. This server will expose Prometheus metrics
	// on the specified metrics port.
	// It will also be gracefully shut down on interrupt signals.
	// The metrics server is optional and only started if enabled.
	// It uses the same wait group to manage its lifecycle.
	var metricsServer *http.Server
	if serverConfig.MetricsEnabled && server.MetricsManager != nil {
		metricsServer = serveMetrics(server, serverConfig)
	}

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal to gracefully shut down the server
	select {
	case chErr := <-serverErrChan:
		cancel()
		return chErr
	case <-sigChan:
		attemptGracefulShutdown(webServer, metricsServer, server, cancel)
	}

	return nil // or return error if startup fails
}

func (s *Server) CloseAllLocalSockets() {
	log.Logger().Info("Closing all local sockets")
	namespaces, err := s.Adapter.GetNamespaces()
	if err != nil {
		log.Logger().Warnf("failed to get namespaces: %s", err.Error())
		return
	}

	if len(namespaces) == 0 {
		return
	}
	wg := &sync.WaitGroup{}

	for _, ns := range namespaces {
		sockets := ns.GetSockets()
		if len(sockets) == 0 {
			continue
		}
		for _, socket := range sockets {
			wg.Add(1)
			go func(socket *WebSocket) {
				defer wg.Done()
				socket.Close()
			}(socket)
		}
	}
	wg.Wait()
	log.Logger().Debug("All local sockets closed. Clearing namespaces")
	s.Adapter.ClearNamespaces()
}

func loadAppManager(_ context.Context, conf *config.ServerConfig) (apps.AppManagerInterface, error) {
	var appManager apps.AppManagerInterface
	var err error

	if conf == nil {
		return nil, errors.New("server config is nil")
	}

	switch conf.AppManager {
	case "array":
		arr := &ArrayAppManager{}
		if err = arr.Init(conf.Applications); err != nil {
			return nil, err
		}
		appManager = arr

	default:
		arr := &ArrayAppManager{}
		if err = arr.Init(conf.Applications); err != nil {
			return nil, err
		}
		appManager = arr
	}

	return appManager, nil
}

func loadAdapter(ctx context.Context, conf *config.ServerConfig, metricsManager metrics.MetricsInterface) (AdapterInterface, error) {
	var adapter AdapterInterface
	var err error

	if conf == nil {
		return nil, errors.New("server config is nil")
	}

	switch conf.AdapterDriver {
	case "redis":
		if conf.RedisInstance == nil {
			return nil, errors.New("redis instance not configured")
		}
		adapter, err = NewRedisAdapter(ctx, conf.RedisInstance.Client, conf.RedisPrefix, "int", metricsManager)
		if err != nil {
			return nil, err
		}
	case "local":
		adapter = NewLocalAdapter(metricsManager)
		err = adapter.Init()
		if err != nil {
			return nil, err
		}
	default:
		log.Logger().Warnf("unknown adapter driver '%s', defaulting to 'local'", conf.AdapterDriver)
		adapter = NewLocalAdapter(metricsManager)
		err = adapter.Init()
		if err != nil {
			return nil, err
		}
	}
	return adapter, nil
}

func loadQueueManager(ctx context.Context, conf *config.ServerConfig, webhookSender *webhooks.WebhookSender) (queues.QueueInterface, error) {
	var queueManager queues.QueueInterface
	var err error

	if conf == nil {
		return nil, errors.New("server config is nil")
	}

	switch conf.QueueDriver {
	case "redis":
		if conf.RedisInstance == nil {
			return nil, errors.New("redis instance not configured")
		}
		queueManager, err = queues.NewRedisQueue(ctx, conf.RedisInstance, conf.RedisPrefix, webhookSender)
		if err != nil {
			return nil, err
		}
	default:
		queueManager, err = queues.NewSyncQueue(ctx, webhookSender)
		if err != nil {
			return nil, err
		}
	}
	return queueManager, nil
}

func loadCacheManager(ctx context.Context, conf *config.ServerConfig) (cache.CacheContract, error) {
	var cacheManager cache.CacheContract
	var err error

	if conf == nil {
		return nil, errors.New("server config is nil")
	}

	switch conf.ChannelCacheDriver {
	case "redis":
		if conf.RedisInstance == nil {
			return nil, errors.New("redis instance not configured")
		}
		cacheManager = &cache.RedisCache{
			Client: conf.RedisInstance.Client,
			Prefix: conf.RedisPrefix,
		}
	default:
		cacheManager = &cache.LocalCache{}
	}

	err = cacheManager.Init(ctx)
	if err != nil {
		return nil, err
	}
	return cacheManager, nil
}

// handlePanic recovers from panics, logs the error, attempts to close all local sockets,
// and exits the application.
func handlePanic(server *Server) {
	if r := recover(); r != nil {
		log.Logger().Error("Recovered from panic", r)
		server.Closing = true
		attemptGracefulShutdown(nil, nil, server, func() {})
	}
}

// attemptGracefulShutdown listens for OS interrupt signals (like Ctrl+C) and initiates a graceful shutdown
func attemptGracefulShutdown(webServer *http.Server, metricsServer *http.Server, server *Server, cancel context.CancelFunc) {
	log.Logger().Infoln("Received interrupt signal, waiting for all tasks to complete...")

	serverShutdownContext, ctxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer ctxCancel()

	var wg sync.WaitGroup
	server.Closing = true

	// Shutdown the HTTP server

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Logger().Info("Shutting down HTTP server...")
		if err := webServer.Shutdown(serverShutdownContext); err != nil {
			log.Logger().Errorf("HTTP server shutdown error: %v", err)
		} else {
			log.Logger().Info("HTTP server shut down gracefully")
		}
	}()

	// Shutdown the metrics server (if it was started)
	if metricsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Logger().Info("Shutting down Metrics server...")
			if err := metricsServer.Shutdown(serverShutdownContext); err != nil {
				log.Logger().Errorf("Metrics server shutdown error: %v", err)
			} else {
				log.Logger().Info("Metrics server shut down gracefully")
			}
		}()
	}

	// close all local sockets
	socketsClosed := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Logger().Infoln("... closing all local sockets")

		server.CloseAllLocalSockets()
		log.Logger().Infoln("... all local sockets closed")
		close(socketsClosed)
	}()

	// shut down cache manager
	if server.CacheManager != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.CacheManager.Shutdown()
		}()
	}

	if server.QueueManager != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-socketsClosed
			log.Logger().Info("Shutting down Queue manager...")
			queueShutdownCtx, queueCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer queueCancel()
			server.QueueManager.Shutdown(queueShutdownCtx)
			log.Logger().Info("Queue manager shut down")
		}()
	}

	// Cancel the context to signal all goroutines to stop
	cancel()

	// wait for all shutdown tasks to complete or timeout
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		log.Logger().Info("All shutdown tasks completed")
	case <-serverShutdownContext.Done():
		log.Logger().Warn("Shutdown tasks did not complete in time")
	}

	os.Exit(0)
}

func serveMetrics(server *Server, serverConfig *config.ServerConfig) *http.Server {
	mux := http.NewServeMux()
	mh := metrics.NewMetricsHandler(server.PrometheusMetrics)
	mux.Handle("/metrics", mh.TextHandler())
	addr := fmt.Sprintf("%s:%s", serverConfig.BindAddress, serverConfig.MetricsPort)
	metricsServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Logger().Infof("Starting metrics server on %s:%s/metrics\n", serverConfig.BindAddress, serverConfig.MetricsPort)
		if metricsErr := metricsServer.ListenAndServe(); metricsErr != nil && !errors.Is(metricsErr, http.ErrServerClosed) {
			log.Logger().Fatalf("Failed to start metrics server: %s", metricsErr)
		}
		log.Logger().Infoln("Metrics server stopped")
	}()
	return metricsServer
}
