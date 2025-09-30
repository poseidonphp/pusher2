package config

import (
	"context"
	"errors"

	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"pusher/internal/apps"

	"pusher/internal/cache"
	"pusher/internal/clients"
	"pusher/internal/constants"
	"pusher/log"
)

// Version of the application, set at build time via -ldflags "-X 'internal.Version=x.y.z'"
var Version string = "unknown"

// type AppConfig struct {
// 	AppID     constants.AppID
// 	AppKey    string
// 	AppSecret string
// }

type ServerConfig struct {
	ctx                    *context.Context     `mapstructure:"-"`
	Env                    string               `mapstructure:"app_env"`
	Port                   string               `mapstructure:"port"`
	BindAddress            string               `mapstructure:"bind_address"`
	AppManager             string               `mapstructure:"app_manager"`
	AdapterDriver          string               `mapstructure:"adapter_driver"`
	QueueDriver            string               `mapstructure:"queue_driver"`
	ChannelCacheDriver     string               `mapstructure:"cache_driver"`
	ChannelCacheManager    cache.CacheContract  `mapstructure:"-" json:"-"`
	UsingRedis             bool                 `mapstructure:"-" json:"-"`
	RedisInstance          *clients.RedisClient `mapstructure:"-" json:"-"`
	RedisPrefix            string               `mapstructure:"redis_prefix"`
	LogLevel               string               `mapstructure:"log_level"`
	RedisUrl               string               `mapstructure:"redis_url"`
	RedisTls               bool                 `mapstructure:"redis_tls"`
	RedisClusterMode       bool                 `mapstructure:"redis_cluster_mode"`
	IgnoreLoggerMiddleware bool                 `mapstructure:"ignore_logger_middleware"`
	MetricsEnabled         bool                 `mapstructure:"metrics_enabled"`
	MetricsPort            string               `mapstructure:"metrics_port"`
	Applications           []apps.App           `mapstructure:"applications"`
}

func isTest() bool {
	return strings.HasSuffix(os.Args[0], ".test") || os.Getenv("GO_TEST") != ""
}

func initFlags() {
	// 1) Define flags for config-file and env-file overrides
	pflag.BoolP("version", "v", false, "Print version information and exit")
	pflag.String("config-file", "", "Path to the config file (orverrides CONFIG_FILE env var)")
	pflag.StringP("env-file", "e", "", "path to .env file (defaults to ./.env if present)")

	// 2) Define actual application flags with their defaults
	pflag.String("app-env", "production", "Environment to run the server in (default: production)")
	pflag.IntP("port", "p", 6001, "Port on which to run the server")
	pflag.String("bind-address", "0.0.0.0", "Address on which to bind the server")
	pflag.StringP("adapter-driver", "a", "local", "Adapter driver to use")
	pflag.String("queue-driver", "local", "Queue driver to use")
	pflag.String("cache-driver", "local", "Cache driver to use")
	pflag.String("app-manager", "array", "App manager to use")
	pflag.String("redis-url", "localhost:6379", "URL of the Redis server")
	pflag.String("redis-prefix", "pusher", "Prefix to use for Redis keys")
	pflag.Bool("redis-tls", false, "Use TLS for Redis connection")
	pflag.Bool("redis-cluster", false, "Use Redis cluster mode")
	pflag.String("log-level", "warn", "Log level (trace, debug, info, warn, error)")
	pflag.Bool("ignore-logger-middleware", false, "Ignore logger middleware")
	pflag.Bool("metrics-enabled", true, "Enable metrics collection and endpoints")
	pflag.Int("metrics-port", 6001, "Port on which to run the metrics server")

	// Single app config
	pflag.String("app-id", "", "Default app id")
	pflag.String("app-key", "", "Default app key")
	pflag.String("app-secret", "", "Default app secret")
	pflag.Int("app-activity-timeout", 60, "Default app activity timeout")
	pflag.Int("app-authorization-timeout-seconds", 5, "Default app authorization timeout seconds")
	pflag.Int64("app-max-connections", 0, "Default app max connections")
	pflag.Bool("app-enable-client-messages", false, "Default app enable client messages")
	pflag.Bool("app-enabled", true, "Default app enabled")
	pflag.Int("app-max-backend-events-per-second", 0, "Default app max backend events per second")
	pflag.Int("app-max-client-events-per-second", 0, "Default app max client events per second")
	pflag.Int("app-max-read-requests-per-second", 0, "Default app max read requests per second")
	pflag.Int("app-max-presence-members-per-channel", 100, "Default app max presence members per channel")
	pflag.Int("app-max-presence-member-size-in-kb", 10, "Default app max presence member size in kb")
	pflag.Int("app-max-channel-name-length", 200, "Default app max channel name length")
	pflag.Int("app-max-event-channels-at-once", 10, "Default app max event channels at once")
	pflag.Int("app-max-event-name-length", 200, "Default app max event name length")
	pflag.Int("app-max-event-payload-in-kb", 10, "Default app max event payload in kb")
	pflag.Int("app-max-event-batch-size", 10, "Default app max event batch size")
	pflag.Bool("app-require-channel-authorization", false, "Default app require channel authorization")
	pflag.Bool("app-has-client-event-webhooks", false, "Default app has client event webhooks")
	pflag.Bool("app-has-channel-occupied-webhooks", false, "Default app has channel occupied webhooks")
	pflag.Bool("app-has-channel-vacated-webhooks", false, "Default app has channel vacated webhooks")
	pflag.Bool("app-has-member-added-webhooks", false, "Default app has member added webhooks")
	pflag.Bool("app-has-member-removed-webhooks", false, "Default app has member removed webhooks")
	pflag.Bool("app-has-cache-miss-webhooks", false, "Default app has cache miss webhooks")
	pflag.Bool("app-webhook-batching-enabled", false, "Default app webhook batching enabled")
	pflag.Bool("app-webhooks-enabled", false, "Default app webhooks enabled")
	pflag.String("app-webhook-url", "", "Default app webhook URL")
	pflag.String("app-webhook-sns-region", "us-east-1", "Default app webhook SNS region")
	pflag.String("app-webhook-sns-topic-arn", "", "Default app webhook SNS topic ARN")
	pflag.String("app-webhook-filter-prefix", "", "Default app webhook filter prefix")
	// Parse the flags
	pflag.Parse()

	// Bind some flags to their snake-kebab-case equivalents in viper
	// so that both --app-env and --app_env work
	_ = viper.BindPFlag("app_env", pflag.Lookup("app-env"))
	_ = viper.BindPFlag("bind_address", pflag.Lookup("bind-address"))
	_ = viper.BindPFlag("adapter_driver", pflag.Lookup("adapter-driver"))
	_ = viper.BindPFlag("queue_driver", pflag.Lookup("queue-driver"))
	_ = viper.BindPFlag("cache_driver", pflag.Lookup("cache-driver"))
	_ = viper.BindPFlag("app_manager", pflag.Lookup("app-manager"))
	_ = viper.BindPFlag("redis_url", pflag.Lookup("redis-url"))
	_ = viper.BindPFlag("redis_prefix", pflag.Lookup("redis-prefix"))
	_ = viper.BindPFlag("redis_tls", pflag.Lookup("redis-tls"))
	_ = viper.BindPFlag("redis_cluster", pflag.Lookup("redis-cluster"))
	_ = viper.BindPFlag("log_level", pflag.Lookup("log-level"))
	_ = viper.BindPFlag("ignore_logger_middleware", pflag.Lookup("ignore-logger-middleware"))
	_ = viper.BindPFlag("metrics_enabled", pflag.Lookup("metrics-enabled"))
	_ = viper.BindPFlag("metrics_port", pflag.Lookup("metrics-port"))

}

func InitializeServerConfig(_ *context.Context) (*ServerConfig, error) {
	// Initialize Viper and pflag to read configuration from multiple sources
	initFlags()

	showVersion := pflag.Lookup("version").Value.String() == "true"
	if showVersion {
		fmt.Printf("SocketRush Version: %s\n", Version)
		os.Exit(0)
	}

	// 3) Figure out which .env to load (if any) and load it
	if !isTest() || os.Getenv("TEST_WITH_ENV_FILE") == "true" {
		envFile := pflag.Lookup("env-file").Value.String()
		if envFile == "" {
			envFile = os.Getenv("ENV_FILE")
		}
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

	// 4) Let Viper pick up env vars
	viper.AutomaticEnv()

	// allow keys like "log-level" or "LOG_LEVEL" to map to "log_level"
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// 5) Determine config-file path: CLI flag wins over ENV var
	cfgPath := ""
	if f := pflag.Lookup("config-file").Value.String(); f != "" {
		cfgPath = f
	} else if f = os.Getenv("CONFIG_FILE"); f != "" {
		cfgPath = f
	}
	if cfgPath != "" {
		viper.SetConfigFile(cfgPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %v", err)
		} else {
			log.Logger().Infoln("Using config file:", viper.ConfigFileUsed())
		}
	}

	// bind single-app flags into flat keys
	_ = viper.BindPFlag("app_id", pflag.Lookup("app-id"))
	_ = viper.BindPFlag("app_key", pflag.Lookup("app-key"))
	_ = viper.BindPFlag("app_secret", pflag.Lookup("app-secret"))
	_ = viper.BindPFlag("app_activity_timeout", pflag.Lookup("app-activity-timeout"))
	_ = viper.BindPFlag("app_authorization_timeout_seconds", pflag.Lookup("app-authorization-timeout-seconds"))
	_ = viper.BindPFlag("app_max_connections", pflag.Lookup("app-max-connections"))
	_ = viper.BindPFlag("app_enable_client_messages", pflag.Lookup("app-enable-client-messages"))
	_ = viper.BindPFlag("app_enabled", pflag.Lookup("app-enabled"))
	_ = viper.BindPFlag("app_max_backend_events_per_second", pflag.Lookup("app-max-backend-events-per-second"))
	_ = viper.BindPFlag("app_max_client_events_per_second", pflag.Lookup("app-max-client-events-per-second"))
	_ = viper.BindPFlag("app_max_read_requests_per_second", pflag.Lookup("app-max-read-requests-per-second"))
	_ = viper.BindPFlag("app_max_presence_members_per_channel", pflag.Lookup("app-max-presence-members-per-channel"))
	_ = viper.BindPFlag("app_max_presence_member_size_in_kb", pflag.Lookup("app-max-presence-member-size-in-kb"))
	_ = viper.BindPFlag("app_max_channel_name_length", pflag.Lookup("app-max-channel-name-length"))
	_ = viper.BindPFlag("app_max_event_channels_at_once", pflag.Lookup("app-max-event-channels-at-once"))
	_ = viper.BindPFlag("app_max_event_name_length", pflag.Lookup("app-max-event-name-length"))
	_ = viper.BindPFlag("app_max_event_payload_in_kb", pflag.Lookup("app-max-event-payload-in-kb"))
	_ = viper.BindPFlag("app_max_event_batch_size", pflag.Lookup("app-max-event-batch-size"))
	_ = viper.BindPFlag("app_require_channel_authorization", pflag.Lookup("app-require-channel-authorization"))
	_ = viper.BindPFlag("app_has_client_event_webhooks", pflag.Lookup("app-has-client-event-webhooks"))
	_ = viper.BindPFlag("app_has_channel_occupied_webhooks", pflag.Lookup("app-has-channel-occupied-webhooks"))
	_ = viper.BindPFlag("app_has_channel_vacated_webhooks", pflag.Lookup("app-has-channel-vacated-webhooks"))
	_ = viper.BindPFlag("app_has_member_added_webhooks", pflag.Lookup("app-has-member-added-webhooks"))
	_ = viper.BindPFlag("app_has_member_removed_webhooks", pflag.Lookup("app-has-member-removed-webhooks"))
	_ = viper.BindPFlag("app_has_cache_miss_webhooks", pflag.Lookup("app-has-cache-miss-webhooks"))
	_ = viper.BindPFlag("app_webhook_batching_enabled", pflag.Lookup("app-webhook-batching-enabled"))
	_ = viper.BindPFlag("app_webhooks_enabled", pflag.Lookup("app-webhooks-enabled"))
	_ = viper.BindPFlag("app_webhook_url", pflag.Lookup("app-webhook-url"))
	_ = viper.BindPFlag("app_webhook_sns_region", pflag.Lookup("app-webhook-sns-region"))
	_ = viper.BindPFlag("app_webhook_sns_topic_arn", pflag.Lookup("app-webhook-sns-topic-arn"))
	_ = viper.BindPFlag("app_webhook_filter_prefix", pflag.Lookup("app-webhook-filter-prefix"))

	// 6) Bind all CLI flags into Viper (highest precedence)
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("error binding flags: %v", err)
	}

	// 7) unmarshal the viper config (cli, env, and config params) into ServerConfig struct
	var cfg ServerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	// —–– post‑processing for “single‑app” mode –––
	if len(cfg.Applications) == 0 {
		// if any of the "single-app" flags/ENV were set, grab them:
		id, key, sec := viper.GetString("app_id"), viper.GetString("app_key"), viper.GetString("app_secret")
		activityTimeout := viper.GetInt("app_activity_timeout")
		authorizationTimeoutInSeconds := viper.GetInt("app_authorization_timeout_seconds")
		maxConnections := viper.GetInt64("app_max_connections")
		enableClientMessages := viper.GetBool("app_enable_client_messages")
		enabled := viper.GetBool("app_enabled")
		maxBackendEventsPerSecond := viper.GetInt("app_max_backend_events_per_second")
		maxClientEventsPerSecond := viper.GetInt("app_max_client_events_per_second")
		maxReadRequestsPerSecond := viper.GetInt("app_max_read_requests_per_second")
		maxPresenceMembersPerChannel := viper.GetInt("app_max_presence_members_per_channel")
		maxPresenceMemberSizeInKb := viper.GetInt("app_max_presence_member_size_in_kb")
		maxChannelNameLength := viper.GetInt("app_max_channel_name_length")
		maxEventChannelsAtOnce := viper.GetInt("app_max_event_channels_at_once")
		maxEventNameLength := viper.GetInt("app_max_event_name_length")
		maxEventPayloadInKb := viper.GetInt("app_max_event_payload_in_kb")
		maxEventBatchSize := viper.GetInt("app_max_event_batch_size")
		requireChannelAuthorization := viper.GetBool("app_require_channel_authorization")
		hasClientEventWebhooks := viper.GetBool("app_has_client_event_webhooks")
		hasChannelOccupiedWebhooks := viper.GetBool("app_has_channel_occupied_webhooks")
		hasChannelVacatedWebhooks := viper.GetBool("app_has_channel_vacated_webhooks")
		hasMemberAddedWebhooks := viper.GetBool("app_has_member_added_webhooks")
		hasMemberRemovedWebhooks := viper.GetBool("app_has_member_removed_webhooks")
		hasCacheMissWebhooks := viper.GetBool("app_has_cache_miss_webhooks")
		webhookBatchingEnabled := viper.GetBool("app_webhook_batching_enabled")
		webhooksEnabled := viper.GetBool("app_webhooks_enabled")
		webhookUrl := viper.GetString("app_webhook_url")
		webhookSnsRegion := viper.GetString("app_webhook_sns_region")
		webhookSnsTopicArn := viper.GetString("app_webhook_sns_topic_arn")
		webhookFilterPrefix := viper.GetString("app_webhook_filter_prefix")

		// activityTimeoutInt := int64(math.Round((float64(activityTimeout) * 10.0) / 9.0))
		// readTimeout := time.Duration(activityTimeoutInt) * time.Second
		authorizationTimeout := time.Duration(authorizationTimeoutInSeconds) * time.Second

		if id != "" || key != "" || sec != "" {
			cfg.Applications = append(cfg.Applications, apps.App{
				ID:              id,
				Key:             key,
				Secret:          sec,
				ActivityTimeout: activityTimeout,
				// ReadTimeout:                  readTimeout,
				AuthorizationTimeout:         authorizationTimeout,
				AuthorizationTimeoutSeconds:  authorizationTimeoutInSeconds,
				MaxConnections:               maxConnections,
				EnableClientMessages:         enableClientMessages,
				Enabled:                      enabled,
				MaxBackendEventsPerSecond:    maxBackendEventsPerSecond,
				MaxClientEventsPerSecond:     maxClientEventsPerSecond,
				MaxReadRequestsPerSecond:     maxReadRequestsPerSecond,
				MaxPresenceMembersPerChannel: maxPresenceMembersPerChannel,
				MaxPresenceMemberSizeInKb:    maxPresenceMemberSizeInKb,
				MaxChannelNameLength:         maxChannelNameLength,
				MaxEventChannelsAtOnce:       maxEventChannelsAtOnce,
				MaxEventNameLength:           maxEventNameLength,
				MaxEventPayloadInKb:          maxEventPayloadInKb,
				MaxEventBatchSize:            maxEventBatchSize,
				RequireChannelAuthorization:  requireChannelAuthorization,
				HasClientEventWebhooks:       hasClientEventWebhooks,
				HasChannelOccupiedWebhooks:   hasChannelOccupiedWebhooks,
				HasChannelVacatedWebhooks:    hasChannelVacatedWebhooks,
				HasMemberAddedWebhooks:       hasMemberAddedWebhooks,
				HasMemberRemovedWebhooks:     hasMemberRemovedWebhooks,
				HasCacheMissWebhooks:         hasCacheMissWebhooks,
				WebhookBatchingEnabled:       webhookBatchingEnabled,
				WebhooksEnabled:              webhooksEnabled,
				WebhookURL:                   webhookUrl,
				WebhookSNSRegion:             webhookSnsRegion,
				WebhookSNSTopicARN:           webhookSnsTopicArn,
				WebhookFilterPrefix:          webhookFilterPrefix,
			})
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
		log.Logger().Infoln("Initializing Redis client")
		client := &clients.RedisClient{Prefix: cfg.RedisPrefix}
		err := client.InitRedis(cfg.RedisUrl, cfg.RedisClusterMode, cfg.RedisTls)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error initializing Redis client: %s", err.Error()))
		}
		cfg.RedisInstance = client
	}

	// 10) Set up the logger
	lvl, _ := logrus.ParseLevel(cfg.LogLevel)
	log.Logger().SetLevel(lvl)
	if cfg.Env != constants.PRODUCTION {
		log.Logger().SetFormatter(&logrus.TextFormatter{})
	}

	return &cfg, nil
}
