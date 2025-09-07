[![codecov](https://codecov.io/gh/poseidonphp/pusher2/graph/badge.svg?token=DHCDS0CYOG)](https://codecov.io/gh/poseidonphp/pusher2)

# Socket Rush - Realtime WebSocket Server

Socket Rush is a scalable, multi-node WebSocket server with support for presence channels, private channels, and encrypted channels. It's designed to be flexible and configurable through various methods.

I've tried to build as much feature parity to the Pusher service as possible, but there are some capabilities that haven't been added yet. 
The goal is to provide a self-hosted solution that can be easily deployed and managed.

This service can be used with the pusher client and server SDK's/libraries. For known missing Pusher features, see bottom of this document.

## Configuration Options

Socket Rush can be configured through three methods (in order of precedence):

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **JSON configuration file** (lowest priority by default, but can be preferred over env vars with the `--prefer-config-file` flag)

## Server-Level Command-line Flags

The 3 parameters that are required to run the server are `--app-id`, `--app-key`, and `--app-secret`. These can be set through command-line flags, environment variables, or in the JSON configuration file.

When using a database driven app manager (not yet implemented), those 3 parameters will not be required.

| Flag                                     | Environment Variable                   | Config JSON Key                                       | Config JSON Type | Description                                                            | Default            |
|------------------------------------------|----------------------------------------|-------------------------------------------------------|------------------|------------------------------------------------------------------------|--------------------|
| `--port`                                 | `PORT`                                 | `port`                                                | int              | Port on which to run the server                                        | `6001`             |
| `--bind-address`                         | `BIND_ADDRESS`                         | `bind_address`                                        | string           | Address on which to bind the server                                    | `"0.0.0.0"`        |
| `--env-file`                             | n/a                                    | n/a                                                   | n/a              | Path to the .env file                                                  | `"./.env"`         |
| `--config-file`                          | n/a                                    | n/a                                                   | n/a              | Path to a config json file                                             | None               |
| `--cache-driver`                         | `CACHE_DRIVER`                         | `cache_driver`                                        | string           | Cache driver to use (`local`, `redis`)                                 | `"local"`          |
| `--queue-driver`                         | `QUEUE_DRIVER`                         | `queue_driver`                                        | string           | Queue driver to use (`local`, `redis`)                                 | `"local"`          |
| `--adapter-driver`                       | `ADAPTER_DRIVER`                       | `adapter_driver`                                      | string           | Adapter driver to use (`local`, `redis`)                               | `"local"`          |
| `--app-manager`                          | `APP_MANAGER`                          | `app_manager`                                         | string           | App manager to use (`array`)                                           | `"array"`          |
| `--app-env`                              | `APP_ENV`                              | `app_env`                                             | string           | Environment to run the server in                                       | `"production"`     |
| `--redis-prefix`                         | `REDIS_PREFIX`                         | `redis_prefix`                                        | string           | Prefix to use for Redis keys                                           | `"pusher"`         |
| `--redis-url`                            | `REDIS_URL`                            | `redis_url`                                           | string           | URL of the Redis server                                                | `"localhost:6379"` |
| `--redis-tls`                            | `REDIS_TLS`                            | `redis_tls`                                           | bool             | Use TLS for Redis connection                                           | `false`            |
| `--redis-cluster`                        | `REDIS_CLUSTER`                        | `redis_cluster`                                       | bool             | Use Redis cluster mode                                                 | `false`            |
| `--log-level`                            | `LOG_LEVEL`                            | `log_level`                                           | string           | Log level (`trace`, `debug`, `info`, `warn`, `error`)                  | `"info"`           |
| `--ignore-logger-middleware`             | `IGNORE_LOGGER_MIDDLEWARE`             | `ignore_logger_middleware`                            | bool             | Ignore logger middleware for performance                               | `false`            |
| `--app-id`                               | `APP_ID`                               | `applications[].app_id`                               | string           | App ID for the default app (sent as string, comprised of only numbers) | None               |
| `--app-key`                              | `APP_KEY`                              | `applications[].app_key`                              | string           | App key for the default app                                            | None               |
| `--app-secret`                           | `APP_SECRET`                           | `applications[].app_secret`                           | string           | App secret for the default app                                         | None               |
| `--app-activity-timeout`                 | `APP_ACTIVITY_TIMEOUT`                 | `applications[].app_activity_timeout`                 | int              | App activity timeout (seconds)                                         | `60`               |
| `--app-max-presence-users`               | `APP_MAX_PRESENCE_USERS`               | `applications[].app_max_presence_users`               | int              | App max presence users                                                 | `100`              |
| `--app-require-authorization`            | `APP_REQUIRE_AUTHORIZATION`            | `applications[].app_require_channel_authorization`    | bool             | App require authorization                                              | `false`            |
| `--app-webhook-enabled`                  | `APP_WEBHOOK_ENABLED`                  | `applications[].app_webhooks_enabled`                 | bool             | App webhook enabled                                                    | `false`            |
| `--app-enable-client-messages`           | `APP_ENABLE_CLIENT_MESSAGES`           | `applications[].app_enable_client_messages`           | bool             | App enable client messages                                             | `false`            |
| `--app-authorization-timeout-seconds`    | `APP_AUTHORIZATION_TIMEOUT_SECONDS`    | `applications[].app_authorization_timeout_seconds`    | int              | App authorization timeout (seconds)                                    | `5`                |
| `--app-max-connections`                  | `APP_MAX_CONNECTIONS`                  | `applications[].app_max_connections`                  | int              | App max connections   (0 for unlimited)                                | `0`                |
| `--app-max-backend-events-per-second`    | `APP_MAX_BACKEND_EVENTS_PER_SECOND`    | `applications[].app_max_backend_events_per_second`    | int              | App max backend events per second (0 for unlimited)                    | `0`                |
| `--app-max-client-events-per-second`     | `APP_MAX_CLIENT_EVENTS_PER_SECOND`     | `applications[].app_max_client_events_per_second`     | int              | App max client events per second (0 for unlimited)                     | `0`                |
| `--app-max-read-requests-per-second`     | `APP_MAX_READ_REQUESTS_PER_SECOND`     | `applications[].app_max_read_requests_per_second`     | int              | App max read requests per second (0 for unlimited)                     | `0`                |
| `--app-max-presence-members-per-channel` | `APP_MAX_PRESENCE_MEMBERS_PER_CHANNEL` | `applications[].app_max_presence_members_per_channel` | int              | App max presence members per channel                                   | `100`              |
| `--app-max-presence-member-size-in-kb`   | `APP_MAX_PRESENCE_MEMBER_SIZE_IN_KB`   | `applications[].app_max_presence_member_size_in_kb`   | int              | App max presence member size in KB                                     | `10`               |
| `--app-max-channel-name-length`          | `APP_MAX_CHANNEL_NAME_LENGTH`          | `applications[].app_max_channel_name_length`          | int              | App max channel name length                                            | `200`              |
| `--app-max-event-channels-at-once`       | `APP_MAX_EVENT_CHANNELS_AT_ONCE`       | `applications[].app_max_event_channels_at_once`       | int              | App max event channels at once                                         | `10`               |
| `--app-max-event-name-length`            | `APP_MAX_EVENT_NAME_LENGTH`            | `applications[].app_max_event_name_length`            | int              | App max event name length                                              | `200`              |
| `--app-max-event-payload-in-kb`          | `APP_MAX_EVENT_PAYLOAD_IN_KB`          | `applications[].app_max_event_payload_in_kb`          | int              | App max event payload in KB                                            | `10`               |
| `--app-max-event-batch-size`             | `APP_MAX_EVENT_BATCH_SIZE`             | `applications[].app_max_event_batch_size`             | int              | App max event batch size                                               | `10`               |
| `--app-require-channel-authorization`    | `APP_REQUIRE_CHANNEL_AUTHORIZATION`    | `applications[].app_require_channel_authorization`    | bool             | App require channel authorization                                      | `false`            |
| `--app-has-client-event-webhooks`        | `APP_HAS_CLIENT_EVENT_WEBHOOKS`        | `applications[].app_has_client_event_webhooks`        | bool             | App has client event webhooks                                          | `false`            |
| `--app-has-channel-occupied-webhooks`    | `APP_HAS_CHANNEL_OCCUPIED_WEBHOOKS`    | `applications[].app_has_channel_occupied_webhooks`    | bool             | App has channel occupied webhooks                                      | `false`            |
| `--app-has-channel-vacated-webhooks`     | `APP_HAS_CHANNEL_VACATED_WEBHOOKS`     | `applications[].app_has_channel_vacated_webhooks`     | bool             | App has channel vacated webhooks                                       | `false`            |
| `--app-has-member-added-webhooks`        | `APP_HAS_MEMBER_ADDED_WEBHOOKS`        | `applications[].app_has_member_added_webhooks`        | bool             | App has member added webhooks                                          | `false`            |
| `--app-has-member-removed-webhooks`      | `APP_HAS_MEMBER_REMOVED_WEBHOOKS`      | `applications[].app_has_member_removed_webhooks`      | bool             | App has member removed webhooks                                        | `false`            |
| `--app-has-cache-miss-webhooks`          | `APP_HAS_CACHE_MISS_WEBHOOKS`          | `applications[].app_has_cache_miss_webhooks`          | bool             | App has cache miss webhooks                                            | `false`            |
| `--app-has-webhook-batching-enabled`     | `APP_HAS_WEBHOOK_BATCHING_ENABLED`     | `applications[].app_has_webhook_batching_enabled`     | bool             | App has webhook batching enabled                                       | `false`            |
| `--app-webhooks-enabled`                 | `APP_WEBHOOKS_ENABLED`                 | `applications[].app_webhooks_enabled`                 | bool             | App webhooks enabled                                                   | `false`            |

## JSON Configuration File

The configuration file should be in JSON format and can be specified with the `--config` flag. Here's an example structure:

```json
{
  "server": {
    "adapter_driver": "redis",
    "queue_driver": "redis",
    "cache_driver": "redis",
    "app_manager": "array",
    "port": 6001,
    "bind_address": "0.0.0.0",
    "log_level": "info",
    "app_env": "production",
    "env_file": "./.env",
    "redis_url": "localhost:6379",
    "redis_prefix": "pusher",
    "redis_tls": false,
    "redis_cluster_mode": false
  },
  "applications": [
    {
      "app_id": "123",
      "app_key": "app1key",
      "app_secret": "app1secret",
      "app_activity_timeout": 60,
      "app_max_presence_members_per_channel": 100,
      "app_max_presence_member_size_in_kb": 10,
      "app_require_channel_authorization": false,
      "app_enable_client_messages": true,
      "app_webhooks_enabled": false
    },
    {
      "app_id": "456",
      "app_key": "app2key",
      "app_secret": "app2secret"
    }
  ]
}
```

## Component Descriptions

### Adapter Driver
This manages the backend communication channel for multi-node deployments. While any option works for a single-node setup, redis is required for multiple nodes.

Currently supported drivers:
* **local** - For single-node deployments
* **redis** - For multi-node deployments

### Queue Driver
Manages the preparation and delivery of webhook events. The queue system prevents delays in event delivery and improves system stability under high loads.

Currently supported drivers:
* **local** - Events are sent immediately by the node that received them. Not recommended for production as it may cause processing delays or crashes under high volumes.
* **redis** - Events are sent to a redis queue for processing, enabling better scaling and reliability. Each node monitors the queue with a background process.


### Cache Driver
Manages channel caching for improved performance.

Currently supported drivers:
* **local** - In-memory cache for single node deployments
* **redis** - Distributed cache for multi-node deployments


## Running the Server
```bash
# Basic run with defaults
socket-rush

# Run with specific port and Redis
socket-rush --port 8080 --adapter-driver redis --redis-url redis://my-redis:6379

# Run with a config file
socket-rush --config-file ./config.json

# Use a different .env file
socket-rush --env-file ../.env

# Run with a specific app configuration
socket-rush --app-id 123 --app-key my-key --app-secret my-secret
```

## Still To Do
- [ ] Watchlist events?
- [ ] Implement socket-id exclusion to match official spec https://pusher.com/docs/channels/server_api/excluding-event-recipients/
- [ ] Batch webhooks
- [ ] Tests - so far I have 0 tests
- [ ] NATS support
- [ ] Metrics support (prometheus)
- [ ] Documentation
- [ ] Examples
- [ ] App Manager: Dynamo
- [ ] App Manager: Mysql/Postgres
- [x] SNS support (not yet tested)
- [ ] issue with sending webhooks for private-encrypted channels; fails to decrypt?
- [ ] TLS support?
- [ ] CORS

## Known Missing Pusher Features
- [ ] Batch webhooks
- [ ] Watch lists