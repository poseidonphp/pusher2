package internal

import (
	"context"
	"errors"
	"os"
	"sync"

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

func (s *Server) CloseAllLocalSockets() {
	// implement similar to ws-handler.ts line 231
	log.Logger().Debug("Closing all local sockets")
	namespaces, err := s.Adapter.GetNamespaces()
	if err != nil {
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

	switch conf.AdapterDriver {
	case "redis":
		if conf.RedisInstance == nil {
			return nil, errors.New("redis instance not configured")
		}
		adapter, err = NewRedisAdapter(ctx, conf.RedisInstance.Client, conf.RedisPrefix, "int", metricsManager)
		if err != nil {
			return nil, err
		}
	default:
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
