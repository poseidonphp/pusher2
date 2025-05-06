package internal

import (
	"context"

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
	config         *config.ServerConfig
	ctx            context.Context
	AppManager     apps.AppManagerInterface
	Adapter        AdapterInterface
	MetricsManager metrics.MetricsInterface
	CacheManager   cache.CacheContract
	QueueManager   queues.QueueInterface
	WebhookSender  *webhooks.WebhookSender
	Closing        bool
	// websocketPool  sync.Pool
}

func NewServer(ctx context.Context, conf *config.ServerConfig) (*Server, error) {

	adapter, err := loadAdapter(ctx, conf)
	if err != nil {
		return nil, err
	}

	appManager, err := loadAppManager(ctx, conf)
	if err != nil {
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

	// s.websocketPool = sync.Pool{
	// 	New: func() interface{} {
	// 		return &WebSocket{
	// 			SubscribedChannels: make(map[constants.ChannelName]*Channel),
	// 			PresenceData:       make(map[constants.ChannelName]*pusherClient.MemberData),
	// 		}
	// 	},
	// }

	return s, nil
}

func (s *Server) CloseAllLocalSockets() {
	// implement similar to ws-handler.ts line 231
	log.Logger().Info("Closing all local sockets")
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
	log.Logger().Info("All local sockets closed. Clearing namespaces")
	s.Adapter.ClearNamespaces()
	// runtime.GC()
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

func loadCacheManager(_ context.Context, conf *config.ServerConfig) (cache.CacheContract, error) {
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
