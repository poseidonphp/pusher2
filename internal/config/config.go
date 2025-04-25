package config

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"pusher/env"
	"pusher/internal/cache"
	"pusher/internal/clients"
	"pusher/internal/constants"
	"pusher/log"
)

type AppID = string

type AppConfig struct {
	AppID     AppID
	AppKey    string
	AppSecret string
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

type CommandLineFlags struct {
	DotEnvPath string
	Port       string
}

type ServerConfig struct {
	ctx *context.Context
	// GlobalWaitGroup sync.WaitGroup
	Env         string
	Port        string
	BindAddress string
	// Apps                     map[AppID]AppConfig
	AdapterDriver string
	// WebhookDriver         string
	// WebhookURL            string
	// WebhookEnabled        bool
	// WebhookEvents         []string
	// WebhookChannelsFilter []string
	// WebhookSnsTopicArn    string
	// WebhookSnsRegion      string
	// WebhookManager        webhooks.WebhookInterface
	QueueDriver string
	// QueueManager queues.QueueInterface
	// PubSubDriver  string
	// PubSubManager pubsub.PubSubManagerContract
	// StorageDriver         string
	// StorageManager        storage.StorageContract
	ChannelCacheDriver  string
	ChannelCacheManager cache.CacheContract
	UsingRedis          bool
	RedisInstance       *clients.RedisClient
	RedisPrefix         string
	// ActivityTimeout          int
	// ReadTimeout              time.Duration
	// MaxPresenceUsers         int64
	// MaxPresenceUserDataBytes int
	// HubCleanerInterval       time.Duration
}

// InitializeServerConfig loads environment variables and sets some global variables
func InitializeServerConfig(ctx *context.Context, flags CommandLineFlags) (*ServerConfig, error) {
	// Load .env file if it exists
	if fileExists(flags.DotEnvPath) {
		err := godotenv.Load()
		if err != nil {
			log.Logger().Errorln("Error loading .env file")
			return nil, err
		}
	}

	// Set some default values for the config
	// activityTimeout := env.GetInt("ACTIVITY_TIMEOUT", 60)

	adapterDriver := env.GetString("ADAPTER_MANAGER", "local")

	queueDriver := env.GetString("QUEUE_MANAGER", "local")
	// pubSubDriver := env.GetString("PUBSUB_MANAGER", "local")
	// storageDriver := env.GetString("STORAGE_MANAGER", "local")
	channelCacheDriver := env.GetString("CHANNEL_CACHE_MANAGER", "local")

	port := env.GetString("BIND_PORT", "6001")
	if flags.Port != "" {
		port = flags.Port
	}

	// Create the new config object

	newConfig := &ServerConfig{
		ctx: ctx,
		// GlobalWaitGroup:       sync.WaitGroup{},
		Env:           env.GetString("APP_ENV", constants.PRODUCTION),
		Port:          port,
		BindAddress:   env.GetString("BIND_ADDR", "0.0.0.0"),
		AdapterDriver: adapterDriver,
		// WebhookDriver:         env.GetString("WEBHOOK_MANAGER", "http"),
		// WebhookURL:            env.GetString("WEBHOOK_URL", ""),
		// WebhookEnabled:        env.GetBool("WEBHOOK_ENABLED", false),
		// WebhookEvents:         []string{},
		// WebhookChannelsFilter: []string{},
		// WebhookSnsRegion:      env.GetString("SNS_REGION", ""),
		// WebhookSnsTopicArn:    env.GetString("SNS_TOPIC_ARN", ""),
		QueueDriver: queueDriver,
		// PubSubDriver:          pubSubDriver,
		// StorageDriver:         storageDriver,
		ChannelCacheDriver: channelCacheDriver,
		UsingRedis:         queueDriver == "redis" || adapterDriver == "redis" || channelCacheDriver == "redis",
		RedisPrefix:        env.GetString("REDIS_PREFIX", "pusher"),
		// ActivityTimeout:          activityTimeout,
		// ReadTimeout:              time.Duration(float64(activityTimeout)*10.0/9.0) * time.Second,
		// MaxPresenceUsers:         env.GetInt64("MAX_PRESENCE_USERS", 100),
		// MaxPresenceUserDataBytes: env.GetInt("MAX_PRESENCE_USER_DATA_KB", 10) * 1024,
		// HubCleanerInterval:       env.GetSeconds("CLEANER_INTERVAL", 60),
	}

	// Set up application(s)
	// err := newConfig.setupApps()
	// if err != nil {
	// 	return nil, errors.New(fmt.Sprintf("Error setting up apps: %v", err.Error()))
	// }

	// Set up Redis if needed
	if newConfig.UsingRedis {
		client := &clients.RedisClient{Prefix: newConfig.RedisPrefix}
		err := client.InitRedis(env.GetString("REDIS_URL", "xxx"), env.GetBool("REDIS_CLUSTER", false), env.GetBool("REDIS_USE_TLS", false))
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error initializing Redis client: %v", err.Error()))
		}
		newConfig.RedisInstance = client
	}

	// Set up the logger
	lvl, _ := logrus.ParseLevel(env.GetString("LOG_LEVEL", "info"))
	log.Logger().SetLevel(lvl)
	if newConfig.Env != constants.PRODUCTION {
		log.Logger().SetFormatter(&logrus.TextFormatter{})
	}

	// Output the configurations
	log.Logger().Debugln("AppEnv:", newConfig.Env)
	log.Logger().Debugln("LogLevel", env.Get("LOG_LEVEL", "info"))

	return newConfig, nil
}

func ParseCommandLineFlags() CommandLineFlags {
	portFlag := flag.String("port", "", "port on which to run the server")
	envPathFlag := flag.String("env-file", "./.env", "path of the .env file (default: ./.env)")
	flag.Parse()

	return CommandLineFlags{
		DotEnvPath: *envPathFlag,
		Port:       *portFlag,
	}
}

// setupApps initializes the apps map with the app ID, key, and secret from the environment variables
// but eventually could be replaced with a database call
// func (s *ServerConfig) setupApps() error {
// 	s.Apps = make(map[AppID]AppConfig)
// 	appID := env.GetString("APP_ID", "")
// 	appKey := env.GetString("APP_KEY", "")
// 	appSecret := env.GetString("APP_SECRET", "")
//
// 	if !util.ValidAppID(appID) || appID == "" {
// 		return errors.New("invalid app id")
// 	}
//
// 	if appKey == "" || appSecret == "" {
// 		return errors.New("missing app key/secret")
// 	}
//
// 	s.Apps[appID] = AppConfig{
// 		AppID:     appID,
// 		AppKey:    appKey,
// 		AppSecret: appSecret,
// 	}
// 	return nil
// }

func (s *ServerConfig) InitializeBackendServices() error {
	// // Initialize the webhook manager
	// err := s.InitializeWebhookManager()
	// if err != nil {
	// 	return errors.New(fmt.Sprintf("failed to initialize webhook manager: %v", err.Error()))
	// }
	//
	// // Initialize the dispatcher
	// if s.WebhookEnabled {
	// 	err = s.InitializeDispatcher()
	// 	if err != nil {
	// 		return errors.New(fmt.Sprintf("failed to initialize dispatcher: %v", err.Error()))
	// 	}
	// }
	//
	// // Initialize the pubsub manager
	// err = s.InitializePubSubManager()
	// if err != nil {
	// 	return errors.New(fmt.Sprintf("failed to initialize pubsub manager: %v", err.Error()))
	// }

	// // Initialize the storage manager
	// err = s.InitializeStorageManager()
	// if err != nil {
	// 	return errors.New(fmt.Sprintf("failed to initialize storage manager: %v", err.Error()))
	// }
	// s.StorageManager.Start()

	// Initialize the channel cache manager
	err := s.InitializeChannelCacheManager()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to initialize channel cache manager: %v", err.Error()))
	}
	return nil
}

// InitializeWebhookManager initializes the webhook manager based on the configuration
// The WebhookManager is the component that actually sends the message back to the client
// func (s *ServerConfig) InitializeWebhookManager() error {
// 	if s.WebhookDriver == "http" {
// 		log.Logger().Debugln("Webhook driver is HTTP")
// 		s.WebhookManager = &webhooks.HttpWebhook{
// 			WebhookUrl: s.WebhookURL,
// 		}
// 	} else if s.WebhookDriver == "sns" {
// 		log.Logger().Debugln("Webhook driver is SNS")
// 		s.WebhookManager = &webhooks.SnsWebhook{
// 			TopicArn: s.WebhookSnsTopicArn,
// 			Region:   s.WebhookSnsRegion,
// 		}
// 	} else {
// 		return errors.New("invalid webhook driver: " + s.WebhookDriver)
// 	}
// 	return nil
// }

// func (s *ServerConfig) InitializeDispatcher() error {
// 	switch s.QueueDriver {
// 	case "local":
// 		log.Logger().Debugln("Dispatcher driver is local")
// 		s.QueueManager = &queues.SyncQueue{WebhookManager: s.WebhookManager}
// 	case "redis":
// 		log.Logger().Debugln("Dispatcher driver is Redis")
// 		s.QueueManager = &queues.RedisQueue{
// 			RedisClient:    s.RedisInstance,
// 			WebhookManager: s.WebhookManager,
// 		}
// 	default:
// 		return errors.New("invalid dispatcher manager: " + s.QueueDriver)
// 	}
//
// 	err := s.QueueManager.Init()
// 	if err != nil {
// 		return errors.New(fmt.Sprintf("failed to initialize dispatcher: %v", err.Error()))
// 	}
//
// 	s.QueueManager.InitFlapDetection(s.WebhookEnabled, s.QueueManager, env.GetInt("FLAP_DETECTION_THRESHOLD", 3))
//
// 	s.GlobalWaitGroup.Add(1)
// 	go func() {
// 		defer s.GlobalWaitGroup.Done()
// 		s.QueueManager.ListenForEvents(*s.ctx)
// 	}()
// 	return nil
// }

// func (s *ServerConfig) InitializePubSubManager() error {
// 	switch s.PubSubDriver {
// 	case "local":
// 		log.Logger().Debugln("PubSub driver is local")
// 		s.PubSubManager = &pubsub.StandAlonePubSubManager{}
// 	case "redis":
// 		log.Logger().Debugln("PubSub driver is Redis")
// 		s.PubSubManager = &pubsub.RedisPubSubManager{
// 			Client:    s.RedisInstance.Client,
// 			KeyPrefix: s.RedisInstance.Prefix,
// 		}
// 	default:
// 		return errors.New("invalid pubsub manager: " + s.PubSubDriver)
// 	}
//
// 	err := s.PubSubManager.Init()
// 	if err != nil {
// 		return errors.New(fmt.Sprintf("failed to initialize pubsub manager: %v", err.Error()))
// 	}
//
// 	return nil
// }

// func (s *ServerConfig) InitializeStorageManager() error {
// 	switch s.StorageDriver {
// 	case "local":
// 		log.Logger().Debugln("Storage driver is local")
// 		s.StorageManager = &storage.StandaloneStorageManager{Pubsub: s.PubSubManager}
// 	case "redis":
// 		log.Logger().Debugln("Storage driver is Redis")
// 		s.StorageManager = &storage.RedisStorage{
// 			Client:    s.RedisInstance.Client,
// 			KeyPrefix: s.RedisInstance.Prefix,
// 		}
// 	case "default":
// 		return errors.New("invalid storage driver: " + s.StorageDriver)
// 	}
//
// 	err := s.StorageManager.Init()
// 	if err != nil {
// 		return errors.New(fmt.Sprintf("failed to initialize storage manager: %v", err.Error()))
// 	}
//
// 	return nil
// }

func (s *ServerConfig) InitializeChannelCacheManager() error {
	switch s.ChannelCacheDriver {
	case "local":
		s.ChannelCacheManager = &cache.LocalCache{}
	case "redis":
		s.ChannelCacheManager = &cache.RedisCache{
			Client: s.RedisInstance.Client,
			Prefix: s.RedisInstance.Prefix,
		}
	default:
		return errors.New("invalid channel cache driver: " + s.ChannelCacheDriver)
	}

	err := s.ChannelCacheManager.Init()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to initialize channel cache manager: %v", err.Error()))
	}
	return nil
}

// func (s *ServerConfig) LoadAppByID(appID string) (*AppConfig, error) {
// 	app, ok := s.Apps[appID]
// 	if !ok {
// 		return nil, errors.New("app not found by id: " + appID)
// 	}
// 	return &app, nil
// }
//
// func (s *ServerConfig) LoadAppByKey(appKey string) (*AppConfig, error) {
// 	for _, app := range s.Apps {
// 		if app.AppKey == appKey {
// 			return &app, nil
// 		}
// 	}
// 	return nil, errors.New("app not found by key: " + appKey)
// }
