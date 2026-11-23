package config

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"pusher/internal/apps"
	"pusher/internal/logger"
	"pusher/internal/settings"

	"pusher/internal/cache"
	"pusher/internal/clients"
	"pusher/internal/constants"
)

// Version of the application, set at build time via -ldflags "-X 'internal.Version=x.y.z'"
var Version string = "unknown"

// type AppConfig struct {
// 	AppID     constants.AppID
// 	AppKey    string
// 	AppSecret string
// }

type ServerConfig struct {
	ctx                    *context.Context        `config:"-"`
	Env                    string                  `config:"app_env"`
	Port                   string                  `config:"port"`
	BindAddress            string                  `config:"bind_address"`
	AppManager             string                  `config:"app_manager"`
	AdapterDriver          string                  `config:"adapter_driver"`
	QueueDriver            string                  `config:"queue_driver"`
	ChannelCacheDriver     string                  `config:"cache_driver"`
	ChannelCacheManager    cache.CacheContract     `config:"-" json:"-"`
	UsingRedis             bool                    `config:"-" json:"-"`
	RedisInstance          *clients.RedisClient    `config:"-" json:"-"`
	RedisPrefix            string                  `config:"redis_prefix"`
	LogLevel               string                  `config:"log_level"`
	Logger                 logger.LoggingInterface `config:"-" json:"-"`
	RedisUrl               string                  `config:"redis_url"`
	RedisTls               bool                    `config:"redis_tls"`
	RedisClusterMode       bool                    `config:"redis_cluster"`
	UsingNats              bool                    `config:"-" json:"-"`
	NatsInstance           *clients.NatsClient     `config:"-" json:"-"`
	NatsUrl                string                  `config:"nats_url"`
	NatsPrefix             string                  `config:"nats_prefix"`
	NatsTls                bool                    `config:"nats_tls"`
	IgnoreLoggerMiddleware bool                    `config:"ignore_logger_middleware"`
	MetricsEnabled         bool                    `config:"metrics_enabled"`
	MetricsPort            string                  `config:"metrics_port"`
	Applications           []apps.App              `config:"applications"`
}

func isTest() bool {
	return strings.HasSuffix(os.Args[0], ".test") || os.Getenv("GO_TEST") != ""
}

// defineFlags registers every command-line flag. Each flag name doubles as a
// settings key: "--app-env" is also read from the APP_ENV env var and the
// "app_env" config file key.
func defineFlags(s *settings.Settings) {
	// Flags for config-file and env-file overrides
	s.Bool("version", "v", false, "Print version information and exit")
	s.String("config-file", "", "", "Path to the config file (overrides CONFIG_FILE env var)")
	s.String("env-file", "e", "", "path to .env file (defaults to ./.env if present)")

	// Application flags with their defaults
	s.String("app-env", "", "production", "Environment to run the server in")
	s.Int("port", "p", 6001, "Port on which to run the server")
	s.String("bind-address", "", "0.0.0.0", "Address on which to bind the server")
	s.String("adapter-driver", "a", "local", "Adapter driver to use")
	s.String("queue-driver", "", "local", "Queue driver to use")
	s.String("cache-driver", "", "local", "Cache driver to use")
	s.String("app-manager", "", "array", "App manager to use")
	s.String("redis-url", "", "localhost:6379", "URL of the Redis server")
	s.String("redis-prefix", "", "pusher", "Prefix to use for Redis keys")
	s.Bool("redis-tls", "", false, "Use TLS for Redis connection")
	s.Bool("redis-cluster", "", false, "Use Redis cluster mode")
	s.String("nats-url", "", "nats://localhost:4222", "URL of the NATS server")
	s.String("nats-prefix", "", "pusher", "Prefix to use for NATS subjects and KV bucket names")
	s.Bool("nats-tls", "", false, "Use TLS for NATS connection")
	s.String("log-level", "", "warn", "Log level (trace, debug, info, warn, error)")
	s.Bool("ignore-logger-middleware", "", false, "Ignore logger middleware")
	s.Bool("metrics-enabled", "", false, "Enable metrics collection and endpoints")
	s.Int("metrics-port", "", 6002, "Port on which to run the metrics server")

	// Single app config
	s.String("app-id", "", "", "Default app id")
	s.String("app-key", "", "", "Default app key")
	s.String("app-secret", "", "", "Default app secret")
	s.Int("app-activity-timeout", "", 60, "Default app activity timeout")
	s.Int("app-authorization-timeout-seconds", "", 5, "Default app authorization timeout seconds")
	s.Int64("app-max-connections", "", 0, "Default app max connections")
	s.Bool("app-enable-client-messages", "", false, "Default app enable client messages")
	s.Bool("app-enabled", "", true, "Default app enabled")
	s.Int("app-max-backend-events-per-second", "", 0, "Default app max backend events per second")
	s.Int("app-max-client-events-per-second", "", 0, "Default app max client events per second")
	s.Int("app-max-read-requests-per-second", "", 0, "Default app max read requests per second")
	s.Int("app-max-presence-members-per-channel", "", 100, "Default app max presence members per channel")
	s.Int("app-max-presence-member-size-in-kb", "", 10, "Default app max presence member size in kb")
	s.Int("app-max-channel-name-length", "", 200, "Default app max channel name length")
	s.Int("app-max-event-channels-at-once", "", 10, "Default app max event channels at once")
	s.Int("app-max-event-name-length", "", 200, "Default app max event name length")
	s.Int("app-max-event-payload-in-kb", "", 10, "Default app max event payload in kb")
	s.Int("app-max-event-batch-size", "", 10, "Default app max event batch size")
	s.Bool("app-require-channel-authorization", "", false, "Default app require channel authorization")
	s.Bool("app-has-client-event-webhooks", "", false, "Default app has client event webhooks")
	s.Bool("app-has-channel-occupied-webhooks", "", false, "Default app has channel occupied webhooks")
	s.Bool("app-has-channel-vacated-webhooks", "", false, "Default app has channel vacated webhooks")
	s.Bool("app-has-member-added-webhooks", "", false, "Default app has member added webhooks")
	s.Bool("app-has-member-removed-webhooks", "", false, "Default app has member removed webhooks")
	s.Bool("app-has-cache-miss-webhooks", "", false, "Default app has cache miss webhooks")
	s.Bool("app-webhook-batching-enabled", "", false, "Default app webhook batching enabled")
	s.Bool("app-webhooks-enabled", "", false, "Default app webhooks enabled")
	s.String("app-webhook-url", "", "", "Default app webhook URL")
	s.String("app-webhook-sns-region", "", "us-east-1", "Default app webhook SNS region")
	s.String("app-webhook-sns-topic-arn", "", "", "Default app webhook SNS topic ARN")
	s.String("app-webhook-filter-prefix", "", "", "Default app webhook filter prefix")
}

// InitializeServerConfig builds the server configuration from command-line
// flags (os.Args), environment variables, and an optional JSON config file.
func InitializeServerConfig(_ *context.Context) (*ServerConfig, error) {
	args := os.Args[1:]
	if isTest() {
		// The test binary's own -test.* flags are not ours to parse.
		args = nil
	}
	return loadServerConfig(args)
}

func loadServerConfig(args []string) (*ServerConfig, error) {
	// 1) Define and parse command-line flags
	s := settings.New("socketrush")
	defineFlags(s)
	if err := s.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}

	// Only an explicit --version counts; a VERSION env var must not trigger an exit.
	if s.Changed("version") && s.GetBool("version") {
		fmt.Printf("SocketRush Version: %s\n", Version)
		os.Exit(0)
	}

	// Create the logger instance
	logInstance := logger.NewAppLogger(logger.WithLogLevel("info"), logger.WithFieldColors(map[string]string{
		"channel": "blue",
		"event":   "green",
	}))

	// 2) Figure out which .env to load (if any) and load it.
	// The --env-file flag wins over the ENV_FILE env var.
	if !isTest() || os.Getenv("TEST_WITH_ENV_FILE") == "true" {
		envFile := s.GetString("env_file")
		if envFile == "" {
			envFile = "./.env"
		}

		if _, err := os.Stat(envFile); err == nil {
			if err = godotenv.Load(envFile); err != nil {
				return nil, fmt.Errorf("error loading .env file (%s): %s", envFile, err.Error())
			}
		} else if isTest() {
			return nil, fmt.Errorf("error finding .env file (%s): %s", envFile, err.Error())
		}
	}

	// 3) Load the config file, if any. The --config-file flag wins over the
	// CONFIG_FILE env var (which may come from the .env file loaded above).
	if cfgPath := s.GetString("config_file"); cfgPath != "" {
		if err := s.ReadConfigFile(cfgPath); err != nil {
			return nil, fmt.Errorf("error reading config file: %v", err)
		}
		logInstance.Infof("Using config file: %s\n", cfgPath)
	}

	// 4) Resolve every setting (flag > env > config file > flag default) into ServerConfig
	var cfg ServerConfig
	if err := s.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	// —–– post‑processing for “single‑app” mode –––
	if len(cfg.Applications) == 0 {
		// if any of the "single-app" flags/ENV were set, use them.
		// apps.App's config tags match the app_* setting keys.
		var newApp apps.App
		if err := s.Unmarshal(&newApp); err != nil {
			return nil, fmt.Errorf("error unmarshalling single-app config: %v", err)
		}

		if newApp.ID != "" || newApp.Key != "" || newApp.Secret != "" {
			newApp.AuthorizationTimeout = time.Duration(newApp.AuthorizationTimeoutSeconds) * time.Second

			if newApp.WebhookURL != "" || newApp.WebhookSNSTopicARN != "" {
				newApp.Webhooks = make([]constants.Webhook, 0)
				eventTypes := make([]string, 0)
				if newApp.HasClientEventWebhooks {
					eventTypes = append(eventTypes, "client_event")
				}
				if newApp.HasChannelOccupiedWebhooks {
					eventTypes = append(eventTypes, "channel_occupied")
				}
				if newApp.HasChannelVacatedWebhooks {
					eventTypes = append(eventTypes, "channel_vacated")
				}
				if newApp.HasMemberAddedWebhooks {
					eventTypes = append(eventTypes, "member_added")
				}
				if newApp.HasMemberRemovedWebhooks {
					eventTypes = append(eventTypes, "member_removed")
				}
				if newApp.HasCacheMissWebhooks {
					eventTypes = append(eventTypes, "cache_miss")
				}
				if len(eventTypes) == 0 {
					eventTypes = append(eventTypes, "*")
				}
				newWebhook := constants.Webhook{
					URL:        newApp.WebhookURL,
					EventTypes: eventTypes,
					Filter: constants.WebhookFilters{
						ChannelNameStartsWith: newApp.WebhookFilterPrefix,
					},
					SNSTopicARN: newApp.WebhookSNSTopicARN,
				}
				newApp.Webhooks = append(newApp.Webhooks, newWebhook)
			}

			cfg.Applications = append(cfg.Applications, newApp)

		}
	}

	if len(cfg.Applications) == 0 {
		return nil, errors.New("no applications configured; please configure at least one application via config file, env vars, or CLI flags")
	}

	for i := range cfg.Applications {
		// set app defaults
		cfg.Applications[i].SetMissingDefaults()
	}

	// 8) If any of the drivers are set to redis, we will set UsingRedis to true so it can be initialized
	if cfg.ChannelCacheDriver == "redis" || cfg.QueueDriver == "redis" || cfg.AdapterDriver == "redis" {
		cfg.UsingRedis = true
	} else {
		cfg.UsingRedis = false
	}

	// 9) If UsingRedis is true, initialize the Redis client
	// and assign it to RedisInstance
	if cfg.UsingRedis {
		logInstance.Info("Initializing Redis client\n")
		client := &clients.RedisClient{Prefix: cfg.RedisPrefix}
		err := client.InitRedis(cfg.RedisUrl, cfg.RedisClusterMode, cfg.RedisTls)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error initializing Redis client: %s", err.Error()))
		}
		cfg.RedisInstance = client
	}

	// 9b) Same as above, but for NATS (adapter and/or cache driver)
	if cfg.ChannelCacheDriver == "nats" || cfg.AdapterDriver == "nats" {
		cfg.UsingNats = true
	} else {
		cfg.UsingNats = false
	}

	if cfg.UsingNats {
		logInstance.Info("Initializing NATS client\n")
		natsClient := &clients.NatsClient{Prefix: cfg.NatsPrefix}
		err := natsClient.InitNats(cfg.NatsUrl, cfg.NatsTls)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error initializing NATS client: %s", err.Error()))
		}
		cfg.NatsInstance = natsClient
	}

	// 10) Set the logger level based on the configuration
	logInstance.SetLogLevel(cfg.LogLevel)
	cfg.Logger = logInstance

	return &cfg, nil
}
