package internal

// This will replace the hub

import (
	"context"

	"pusher/internal/apps"
	"pusher/internal/cache"
	"pusher/internal/config"
	"pusher/internal/metrics"
	"pusher/internal/queues"
	"pusher/internal/webhooks"
)

type Server struct {
	config         *config.ServerConfig
	ctx            context.Context
	AppManager     apps.AppManagerContract
	Adapter        AdapterInterface
	MetricsManager metrics.MetricsInterface
	CacheManager   cache.CacheContract
	QueueManager   queues.QueueInterface
	WebhookSender  *webhooks.WebhookSender
	Closing        bool
}

func NewServer(ctx context.Context, conf *config.ServerConfig) (*Server, error) {

	adapter, err := loadAdapter(ctx, conf)
	if err != nil {
		return nil, err
	}

	appManager := &apps.ArrayAppManager{}
	if err = appManager.Init(); err != nil {
		return nil, err
	}

	metricsManager := &metrics.PrometheusMetrics{}

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

	webhookSender := &webhooks.WebhookSender{
		HttpSender: &webhooks.HttpWebhook{},
		SNSSender:  &webhooks.SnsWebhook{},
	}

	queueManager, err := loadQueueManager(ctx, conf, webhookSender)
	if err != nil {
		return nil, err
	}

	s := &Server{
		ctx:            ctx,
		config:         conf,
		AppManager:     appManager,
		Adapter:        adapter,
		MetricsManager: metricsManager,
		CacheManager:   cacheManager,
		QueueManager:   queueManager,
	}
	return s, nil
}

func (s *Server) CloseAllLocalSockets() {
	// implement similar to ws-handler.ts line 231
}

func loadAdapter(ctx context.Context, conf *config.ServerConfig) (AdapterInterface, error) {
	var adapter AdapterInterface
	var err error

	switch conf.AdapterDriver {
	case "redis":
		adapter, err = NewRedisAdapter(ctx, conf.RedisInstance.Client, conf.RedisPrefix, "int")
		if err != nil {
			return nil, err
		}
	default:
		adapter = &LocalAdapter{}
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
		cacheManager = &cache.RedisCache{
			Client: conf.RedisInstance.Client,
			Prefix: conf.RedisPrefix,
		}
	default:
		cacheManager = &cache.LocalCache{}
	}

	err = cacheManager.Init()
	if err != nil {
		return nil, err
	}
	return cacheManager, nil
}
