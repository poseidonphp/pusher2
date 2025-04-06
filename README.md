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

## TODO
- [ ] Local storage for multi-node (high availability; not scalability)
- [ ] Tests - so far I have 0 tests
- [ ] ~~NATS support~~
- [ ] Metrics support (prometheus)
- [ ] Documentation
- [ ] Examples
- [ ] App Manager (support multiple apps)
- [ ] SNS support
- [ ] Webhooks support
- [ ] Benchmarking
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
- Dispatcher - used for dispatching webhook events. Not yet implemented. Will support local, SQS
- Webhook - how to send events (not yet implemented); will support http, sns