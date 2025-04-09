This project is no where near ready for use... Please do not use it yet. If you stumble on this and want to contribute, 
please do! I would love to have some help on this project. Feel free to open an issue or PR - though at least in the initial stages, i may be picky on what I want to focus or the direction of the project.

Still in the development phase, though some basic functionality is there. 

## Working (AFAIK)
- [x] Basic presence channel support (join/leave)
- [x] Basic channel support (subscribe/unsubscribe)
- [x] Encrypted channel support
- [x] Private/presence/encrypted channel authentication
- [x] Client side events
- [x] Channel APIs/routes
- [x] Redis pubsub support
- [x] Redis storage support (including cleaning up stale records from crashed nodes)
- [x] Multi-node support (via redis)
- [x] Demo UI for use in development of this server
- [x] Auth server emulator for use in development of this server
- [x] Support for local storage (in-memory) for single node
- [x] Support for local pubsub (in-memory) for single node
- [x] Cache Channels
- [x] Graceful shutdown on SIGTERM
- [x] Webhooks support


## TODO
- [ ] Support for webhook config such as channel and event filters
- [ ] Support for terminating user connections
- [ ] Watchlist events?
- [ ] Implement context watchers/timeouts for more actions either within storageLocal or the hub
- [ ] Implement socket-id exclusion to match official spec https://pusher.com/docs/channels/server_api/excluding-event-recipients/
- [ ] Batch triggers?
- [ ] Batch webhooks
- [ ] Local storage for multi-node (high availability; not scalability)
- [ ] Tests - so far I have 0 tests
- [ ] ~~NATS support~~
- [ ] Metrics support (prometheus)
- [ ] Documentation
- [ ] Examples
- [ ] App Manager (support multiple apps)
- [x] SNS support (not yet tested)
- [ ] Benchmarking
- [ ] issue with sending webhooks for private-encrypted channels; fails to decrypt?
- [ ] TLS support?
- [ ] More stuff i'm sure, will update as i think of it

## Things i'm not happy with...
- Needing the redis cleanup job to run in case a node crashes and leaves stale records in there


This is broken up to be as flexible as possible. The following components are built to an interface (along with supported drivers thus far):
- Storage - this is the main storage interface. This is where all the "shared" data is stored (presence channel members, channel counts)
  - Redis
  - Local (in-memory) (single node only)
  - Local-synced (in-memory) (multi-node) not yet implemented
- PubSub - this is the pubsub interface. This is where all the "shared" data is sent to other nodes (presence channel members, channel counts)
  - Redis
  - Local (single node only)
- App Manager - Store/load list of apps - not yet implemented, so far only one app supported via env vars
- Dispatcher - used for dispatching webhook events. Not yet implemented. Will support local, SQS, Redis Queues
- Webhook - how to send events (not yet implemented); will support http, sns

## Deployment
You will need to set some environment variables to use this.

| Variable         | Description                                                                                                                                                                                                                                                                      |
|------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| APP_ID           | *REQUIRED* **Numeric** The app id to use. This is used to identify the app in the storage and pubsub.                                                                                                                                                                            |
| APP_SECRET       | *REQUIRED* The app secret to use. This is used for authenticating server to server and webhook requests                                                                                                                                                                          |
| APP_KEY          | *REQUIRED*  The app key to use. This is used to identify the app in the storage and pubsub.                                                                                                                                                                                      |
|APP_ENV| *Default: production* The environment to use. This is used to determine the level of output by the web server. [ production \| development ]                                                                                                                                     |
| PUBSUB_MANAGER   | *Default: local* [ redis \| local ]                                                                                                                                                                                                                                              |
| STORAGE_MANAGER  | *Default: redis* [ redis \| local ]                                                                                                                                                                                                                                              |
| REDIS_URL        | The redis URL to use if using redis. Must include the port number (redis://localhost:6379)                                                                                                                                                                                       |
| USE_TLS          | *Default: false* Whether or not the redis connection should use TLS                                                                                                                                                                                                              |
| REDIS_PREFIX    | *Default: pusher* The prefix to use for the redis keys. This is used to prevent key collisions.                                                                                                                                                                                  |
| LOG_LEVEL        | *Default: info* The log level to use. [error \| warn \| info \| debug \| trace ]                                                                                                                                                                                                 |
| ACTIVITY_TIMEOUT | *Default: 60* How often should clients ping the server, in seconds                                                                                                                                                                                                               |
|MAX_PRESENCE_USERS | *Default: 100* The maximum number of users in a presence channel. This is used to limit the number of users in a presence channel. If this is reached, the oldest user will be removed from the channel. This is used to prevent memory leaks and to keep the server responsive. |
|MAX_PRESENCE_USER_DATA_KB| *Default: 10* The maximum size of the user data in a presence channel. This is used to limit the size of the user data in a presence channel. This is used to keep the server responsive.                                                                                        |
|BIND_PORT | *Default: 6001* The port to bind the server to. This is used to connect to the server.                                                                                                                                                                                           |
|WEBHOOK_ENABLED| *Default: false* Whether or not to enable webhooks.                                                                                                                                                                                                                              |
|WEBHOOK_MANAGER| *Default: http* The webhook manager to use. [ http \| sns ]                                                                                                                                                                                                                      |
|WEBHOOK_URL| The URL to send the webhooks to. This is used if using the http webhook manager.                                                                                                                                                                                                 |
|DISPATCHER_MANAGER| *Default: local* The dispatcher manager to use for queueing webhooks to be sent. [ local \| redis ]                                                                                                                                                                              |
|SNS_TOPIC_ARN| *required if using sns* The SNS topic ARN to use if using the sns webhook manager. This is used to send the webhooks to the SNS topic.                                                                                                                                           |
|SNS_REGION| *required if using sns* The AWS region to use if using the sns webhook manager. This is used to send the webhooks to the SNS topic.                                                                                                                                              |
|CHANNEL_CACHE_MANAGER| *Default: local* The channel cache manager to use. [ local \| redis ]                                                                                                                                                                                                            |

More variables will be added as the SNS support is added.

## Components
#### PubSub Manager
This is the backend communication channel, used to support multi-node deployments. You can use any option in a single-node setup, but if you are using multiple nodes, you will need to use redis.

#### Storage Manager
This is used to store the presence channel members and channel counts. This is used to support multi-node deployments. You can use any option in a single-node setup, but if you are using multiple nodes, you will need to use one of the other options.

Currently supported drivers are:
* local - everything is kept in-memory on the node
* redis - data is kept in redis. A background 'cleaner' job will run every 20 seconds to look for any stale nodes and remove them from the redis store, which can happen if a node crashes.

#### Dispatcher
This is used for preparing the webhook events to be sent. It is a means for queueing the delivery of the events. The default option is 'local'.

Currently supported drivers are:
* local - (default) events are sent immediately by the node that received the event. This is not recommended for production use, as it can cause delays in the delivery of the events, or could cause the server to crash under high volumes due to exhausting the channel buffer
* redis - events are sent to a redis queue, which can be processed by a separate worker. This is recommended for production use, as it allows for better scaling and reliability of the event delivery. Every node by default will monitor the queue with a separate background process, processing events one at a time.

NOT YET IMPLEMENTED: When using redis dispatcher, you can choose to run a dedicated worker node that only processes events. 

#### Webhook
How webhooks are sent. The most common use is an HTTP POST request to a URL. See [Pusher docs](https://pusher.com/docs/channels/server_api/webhooks/) for more info about the structure and authentication of these requests.

Currently supported drivers are:
* http - (default) sends a POST request to the URL specified in the webhook config. This is the most common use case, and is recommended for most users.
* sns - send requests as an SNS message, using the same payload as the HTTP requests. This is useful for sending events to AWS services, such as Lambda or SQS. When using SNS, batch events is disabled/ignored. This is not yet implemented, but will be in the future.

## Development
I tried to put as much as I could into a Makefile.

### Starting the websocket server
`make run`

### Starting the development auth server
DO NOT USE THIS IN PRODUCTION, it is for development purposes only.

This server is used to authenticate private/presence channels. 

It will authenticate all requests, using the current minute as the user id (for presence channels).

This server also exposes some endpoints to test requests to the websocket server API for things like channel counts and info, as well as an endpoint for triggering a websocket message as if it came from your internal app.

The server is set to run on port 8099.

To start the auth server, run `make auth`

### Starting the demo UI
`make ui`

This will start a simple UI that connects to the websocket server and allows you to test the server. It will usually run on 5379, but check the console output to confirm.

#### Optional query parameters
You can add some URL query parameters when connecting to the demo UI to test some features.

* Forcing a specific user ID: `user_id=bob`
* Force a specific websocket host/port: `host=localhost` `port=6001`
* example: `http://localhost:5379?user_id=bob&host=localhost&port=6001`

Note: when you specify a user id, the UI will automatically connect a handful of additional channels for that user id

You can use the 📡 button to trigger a message to be sent on that channel, confirming that it is received by all clients that are connected to it.

To test a client side message (whisper), select the radio button next to a connected channel, then type into the whisper box and hit send. Client-side messages are not sent to the current client, so to test this, make sure you have another window open with a user connected to the channel you are testing; you should see the client message on all *other* windows.