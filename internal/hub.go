package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"
	"pusher/env"
	"pusher/internal/cache"
	"pusher/internal/clients"
	"pusher/internal/dispatcher"
	"pusher/internal/pubsub"
	"pusher/internal/storage"
	"pusher/internal/util"
	"sync/atomic"

	"pusher/internal/constants"
	"pusher/log"
	"sync"
	"time"
)

/*
globalChannels json representation: the counts are the number of users in the channel
{
	"channel1": {
		"node1": 50,
		"node2": 100,
	},
}

presenceChannels json representation: the values are the user data (pusherClient.MemberData)
{
	"channel1": {
		"node1": {
			"socket1": {
				"user_id": "user1",
				"user_info": {
					"name": "John Doe",
					"email": "jon@doe.com",
				},
			},
		}
	}
}
*/

var GlobalHub *Hub

var LastHeartbeat atomic.Value

type Hub struct {
	nodeID constants.NodeID

	acceptingNewConnections bool
	stopChan                chan struct{}

	// localChannels stores the sockets that are subscribed to channels on this node
	localChannels map[constants.ChannelName]*Channel

	// sessions store all sessions connected to this instance
	sessions map[constants.SocketID]*Session

	broadcast     chan []byte
	register      chan *Session // channel to handle connecting new sessions
	unregister    chan *Session // channel to handle disconnecting sessions
	server2server chan pubsub.ServerMessage
	mutex         sync.Mutex
	pubsubManager *pubsub.PubSubManagerContract

	cleanerPromotionChannel chan pubsub.ServerMessage
}

// MessageToClient is a struct to send messages to the client - TESTING instead of using individual go routines on each session
type MessageToClient struct {
	Session *Session
	Message []byte
}

func NewHub() *Hub {
	newUuid := uuid.New().String()

	hub := &Hub{
		acceptingNewConnections: true,
		stopChan:                make(chan struct{}),
		nodeID:                  constants.NodeID(fmt.Sprintf("%s%s", newUuid[0:4], newUuid[len(newUuid)-4:])),
		localChannels:           make(map[constants.ChannelName]*Channel),
		sessions:                make(map[constants.SocketID]*Session),
		broadcast:               make(chan []byte),
		register:                make(chan *Session, 100),
		unregister:              make(chan *Session, 100),
		server2server:           make(chan pubsub.ServerMessage),
		cleanerPromotionChannel: make(chan pubsub.ServerMessage),
	}

	pubSubManagerDriver := env.GetString("PUBSUB_MANAGER", "local")
	storageManagerDriver := env.GetString("STORAGE_MANAGER", "local")
	channelCacheManagerDriver := env.GetString("CHANNEL_CACHE_MANAGER", "local")

	if (pubSubManagerDriver == "redis" || storageManagerDriver == "redis" || channelCacheManagerDriver == "redis") && clients.RedisClientInstance == nil {
		rc := &clients.RedisClient{}
		rErr := rc.InitRedis(env.GetString("REDIS_PREFIX", "pusher"))
		if rErr != nil {
			log.Logger().Fatal(rErr)
		}
		clients.RedisClientInstance = rc
	}

	// Initialize the global PubSub Manager, used for broadcasting messages to all nodes
	hub.initializePubSubManager(pubSubManagerDriver)

	// Initialize the global Storage Manager, used for storing channel counts and presence data
	hub.initializeStorageManager(storageManagerDriver)

	// Initialize the global Dispatcher, used for dispatching webhooks
	//hub.initializeDispatcher(dispatchManagerDriver, redisClient)

	hub.initializeCacheManager(channelCacheManagerDriver)

	GlobalHub = hub

	// Register the node with the storage manager
	nErr := storage.Manager.AddNewNode(hub.nodeID)
	if nErr != nil {
		log.Logger().Fatal(nErr)
	}

	return hub
}

func (h *Hub) initializeCacheManager(cacheManagerDriver string) {
	var cacheInstance cache.CacheContract
	switch cacheManagerDriver {
	case "redis":
		cacheInstance = &cache.RedisCache{}
	case "local":
		cacheInstance = &cache.LocalCache{}
	default:
		log.Logger().Fatal("Unsupported cache manager")
		return
	}
	log.Logger().Debugf("Initializing %s cache driver", cacheManagerDriver)
	err := cacheInstance.Init()
	if err != nil {
		log.Logger().Fatalf("Error initializing %s cache: %v", cacheManagerDriver, err)
	}
	cache.ChannelCache = cacheInstance
}

func (h *Hub) initializeStorageManager(storageManagerDriver string) {
	switch storageManagerDriver {
	case "redis":
		storage.Manager = &storage.RedisStorage{
			Client:    clients.RedisClientInstance.GetClient(),
			KeyPrefix: env.GetString("REDIS_PREFIX", "pusher"),
		}
	case "local":
		mgr := &storage.StandaloneStorageManager{}
		_ = mgr.Init()
		//go mgr.ListenForMessages()
		storage.Manager = mgr

	default:
		log.Logger().Fatal("Unsupported storage manager")
	}
}

func (h *Hub) initializePubSubManager(pubSubManagerDriver string) {
	switch pubSubManagerDriver {
	case "redis":
		rPSM := &pubsub.RedisPubSubManager{Client: clients.RedisClientInstance.GetClient()}
		rPSM.SetKeyPrefix(env.GetString("REDIS_PREFIX", "pusher"))
		pubsub.PubSubManager = rPSM
	case "local":
		saPSM := &pubsub.StandAlonePubSubManager{}
		_ = saPSM.Init()
		pubsub.PubSubManager = saPSM
	default:
		log.Logger().Fatal("Unsupported pubsub manager")
	}
}

//func (h *Hub) initializeDispatcher(dispatcherDriver string, redisClient redis.UniversalClient) {
//	switch dispatcherDriver {
//	//case "redis":
//	//	rDispatcher := &dispatcher.RedisDispatcher{Client: redisClient}
//	//	pubsub.Dispatcher = rDispatcher
//	case "local":
//		saDispatcher := &dispatcher.SyncDispatcher{}
//		saDispatcher.Init()
//		dispatcher.Dispatcher = saDispatcher
//	default:
//		log.Logger().Fatal("Unsupported dispatcher manager")
//	}
//}

// startCleaner runs the background process to periodically clean up the globalChannels, presenceChannels, and localChannels maps from stale nodes
func (h *Hub) startCleaner() {
	interval := env.GetSeconds("CLEANER_INTERVAL", 60)
	gracefulExit := false

	log.Logger().Infof("Starting the cleaner - running every %d seconds", int64(interval.Seconds()))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pErr := pubsub.PubSubManager.Publish(ctx, constants.SoketRushInternalChannel, pubsub.ServerMessage{
		NodeID:  GlobalHub.nodeID,
		Event:   constants.SocketRushEventCleanerPromote,
		Payload: []byte{},
	})
	if pErr != nil {
		log.Logger().Errorf("Error publishing cleaner promotion message: %v", pErr)
		return
	}

	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		if !gracefulExit {
			log.Logger().Warnln("Stopping the cleaner")
		}

	}()
	for {
		select {
		case _, ok := <-h.cleanerPromotionChannel:
			if !ok {
				gracefulExit = true
				return
			}
		case <-ticker.C:
			storage.Manager.Cleanup()
		case <-h.stopChan:
			log.Logger().Infoln("... Stopping the cleaner due to shutdown signal")
			gracefulExit = true
			return
		}
	}
}

// Run starts the hub. This is the core function for managing all websocket connections
func (h *Hub) Run() {
	log.Logger().Infoln("Starting the hub")

	// Subscribe to the pubsub channel for server-to-server messages
	go pubsub.PubSubManager.Subscribe(constants.SoketRushInternalChannel, h.server2server) // route incoming messages from pubsub to the server2server channel

	// Start the storage manager
	go storage.Manager.Start()

	// Start the background cleaner process
	time.Sleep(2 * time.Second) // wait for the storage manager to start

	// Start the heartbeat broadcast - this sends out its own heartbeat and checks for other nodes
	go h.broadcastHeartbeat()
	go h.monitorHeartbeat()

	// Star the cleaner process for removing stale nodes
	go h.startCleaner()

	// start listening for hub activity
	for {
		select {
		case msg := <-h.server2server:
			h.handleServerToServerIncoming(msg)
		case session := <-h.register:
			log.Logger().Tracef("[%s]  Hub: Registering new socket id: %s\n", session.socketID, session.socketID)
			h.mutex.Lock()
			h.sessions[session.socketID] = session
			h.mutex.Unlock()
			log.Logger().Tracef("[%s]  Hub: Registered new socket id: %s\n", session.socketID, session.socketID)
		case session := <-h.unregister:
			h.mutex.Lock()
			delete(h.sessions, session.socketID)
			h.mutex.Unlock()
			// TODO: replace with cleanSession() logic
		case <-h.stopChan:
			log.Logger().Infoln("... Stopping the hub due to shutdown signal")
			time.Sleep(1 * time.Second)
			return
		}
	}
}

// function to send heartbeat to other nodes
func (h *Hub) broadcastHeartbeat() {
	log.Logger().Infof("Starting the node heartbeat broadcast; will broadcast every %d seconds", int(storage.NodePingInterval.Seconds()))
	ticker := time.NewTicker(storage.NodePingInterval)
	gracefulExit := false

	defer func() {
		if !gracefulExit {
			log.Logger().Warnln("Stopping the node heartbeat broadcast")
		}

		ticker.Stop()
	}()

	for {
		select {
		case <-ticker.C:
			// Broadcast to others that we're still alive
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Logger().Errorf("Recovered from panic in broadcastHeartbeat: %v", r)
					}
				}()
				t := storage.Manager.SendNodeHeartbeat(h.nodeID)
				if t == nil {
					log.Logger().Errorf("Failed to send heartbeat to storage manager")
					return
				}
				LastHeartbeat.Store(*t)
			}()
		case <-h.stopChan:
			log.Logger().Infoln("... Stopping heartbeat broadcast due to shutdown signal")
			gracefulExit = true
			return
		}
	}
}

func (h *Hub) monitorHeartbeat() {
	log.Logger().Debug("Starting heartbeat monitor")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			last := LastHeartbeat.Load().(time.Time)
			if time.Since(last) > 20*time.Second {
				log.Logger().Warn("Heartbeat appears to have stalled")
				// You could restart the heartbeat goroutine here if needed
			}
		case <-h.stopChan:
			log.Logger().Info("... Stopping heartbeat monitor due to shutdown signal")
			return
		}
	}
}

// sendToOtherServers sends a ServerMessage to all other nodes in the cluster via pubsub
func (h *Hub) sendToOtherServers(msg pubsub.ServerMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := pubsub.PubSubManager.Publish(ctx, constants.SoketRushInternalChannel, msg)
	if err != nil {
		log.Logger().Errorf("Failed to publish message to other servers: %v", err)
	}
}

// handleServerToServerIncoming handles messages received from other nodes in the cluster
// mostly used to broadcast out to connected clients
func (h *Hub) handleServerToServerIncoming(msg pubsub.ServerMessage) {
	switch msg.Event {
	case constants.SocketRushEventCleanerPromote:
		if h.cleanerPromotionChannel != nil && h.nodeID != msg.NodeID {
			close(h.cleanerPromotionChannel)
			h.cleanerPromotionChannel = nil
		}

	case constants.SocketRushEventChannelEvent:
		h.handleIncomingChannelEvent(msg)
	}
}

// addPresenceUser adds the socket to the local connections list, adds the info to redis, and notifies other nodes
func (h *Hub) addPresenceUser(socketID constants.SocketID, channel *Channel, memberData pusherClient.MemberData) {
	log.Logger().Tracef("[%s]  Adding presence user to hub\n", socketID)

	localChannel := h.getOrCreateLocalChannel(channel)

	// add the socket to the local channel
	localChannel.addSocketID(socketID)

	// prepare member data for storage and broadcast
	md, mdErr := json.Marshal(memberData)
	if mdErr != nil {
		log.Logger().Errorf("Error marshalling member data to send to other servers: %s", mdErr)
		return
	}

	// add the user to the pubsub connection
	err := storage.Manager.AddUserToPresence(h.nodeID, channel.Name, socketID, memberData)
	if err != nil {
		log.Logger().Errorf("Error adding user to presence: %s", err)
		return
	}
	evt := pusherClient.WebhookEvent{
		Name:    string(constants.WebHookMemberAdded),
		Channel: string(channel.Name),
		UserID:  memberData.UserID,
	}
	bounceString := fmt.Sprintf("channel:%s:%s", channel.Name, memberData.UserID)
	dispatcher.DispatchBuffer.HandleEvent(bounceString, dispatcher.Connect, evt)

	userSocketIds := storage.GetPresenceSocketsForUserID(channel.Name, memberData.UserID)
	skipBroadcast := false
	if len(userSocketIds) > 1 {
		// if there are other sockets for this user, don't broadcast the message
		skipBroadcast = true
	}

	channelEvent := ChannelEvent{
		Event:   constants.PusherInternalPresenceMemberAdded,
		Channel: channel.Name,
		Data:    string(md),
	}

	s2s := pubsub.ServerMessage{
		NodeID:        GlobalHub.nodeID,
		Event:         constants.SocketRushEventChannelEvent,
		Payload:       channelEvent.ToJSON(),
		SocketID:      socketID,
		SocketIDs:     userSocketIds,
		SkipBroadcast: skipBroadcast,
	}

	// send to all nodes for broadcast to all clients
	h.sendToOtherServers(s2s)
}

// getOrCreateLocalChannel ensures the channel exists in the localChannels map which is used for broadcasting to connected clients
func (h *Hub) getOrCreateLocalChannel(channel *Channel) *Channel {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.localChannels[channel.Name] == nil {
		h.localChannels[channel.Name] = channel
	}
	return h.localChannels[channel.Name]
}

// addSessionToChannel adds a session to a channel on the hub for use when broadcasting messages
// also need to add the session to the pubsub channel count
func (h *Hub) addSessionToChannel(session *Session, channel *Channel) {
	h.getOrCreateLocalChannel(channel).addSocketID(session.socketID)
	err := storage.Manager.AddChannel(channel.Name)
	if err != nil {
		log.Logger().Errorf("Error adding channel to storage manager: %s", err)
	}

	if channel.Type != constants.ChannelTypePresence {
		countErr := storage.Manager.AdjustChannelCount(h.nodeID, channel.Name, 1)
		if countErr != nil {
			log.Logger().Errorf("Error adjusting channel count: %s", countErr)
		}
	}
}

// removeSessionFromChannels removes a session from a channel on the hub, and also removes the session from the pubsub channel list.
// If a channel is empty after removal, the webhook for vacated channel is sent.
// If no channels are provided, the session is removed from all channels.
func (h *Hub) removeSessionFromChannels(session *Session, channels ...constants.ChannelName) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	var channelsToRemove []constants.ChannelName

	if len(channels) > 0 {
		channelsToRemove = channels
	} else {
		// remove from all channels
		//channelsToRemove = make([]constants.ChannelName, len(session.subscriptions))

		for channel := range session.subscriptions {
			//log.Logger().Tracef("gracefully removing session from channel: %s", channel)
			session.Tracef("gracefully removing session from channel: %s", channel)
			channelsToRemove = append(channelsToRemove, channel)
		}
	}

	for _, channel := range channelsToRemove {
		session.Tracef("... ... ... removing session from channel: %s", channel)
		if h.localChannels[channel] != nil {
			delete(h.localChannels[channel].Connections, session.socketID)
		}
		channelObj := h.localChannels[channel]

		if channelObj.Type == constants.ChannelTypePresence {
			presenceData, gErr := storage.Manager.GetPresenceDataForSocket(h.nodeID, channel, session.socketID)
			if gErr != nil {
				log.Logger().Errorf("Error getting presence data for socket: %v", gErr)
				continue
			}
			userSocketIds := storage.GetPresenceSocketsForUserID(channel, presenceData.UserID)
			skipBroadcast := false
			if len(userSocketIds) > 1 {
				// if there are other sockets for this user, don't broadcast the message
				skipBroadcast = true
			}

			rErr := storage.Manager.RemoveUserFromPresence(h.nodeID, channel, session.socketID)
			if rErr != nil {
				log.Logger().Errorf("Error removing user from presence: %s", rErr)
				continue
			}

			evt := pusherClient.WebhookEvent{
				Name:    string(constants.WebHookMemberRemoved),
				Channel: string(channel),
				UserID:  presenceData.UserID,
			}
			bounceString := fmt.Sprintf("channel:%s:%s", channel, presenceData.UserID)
			dispatcher.DispatchBuffer.HandleEvent(bounceString, dispatcher.Disconnect, evt)

			// announce that the user has left the channel
			mrd := &MemberRemovedData{UserID: presenceData.UserID}
			_msg := ChannelEvent{
				Event:   constants.PusherInternalPresenceMemberRemoved,
				Channel: channel,
				Data:    mrd.ToString(),
			}
			GlobalHub.sendToOtherServers(pubsub.ServerMessage{
				NodeID: GlobalHub.nodeID,
				Event:  constants.SocketRushEventChannelEvent,
				//Event:    constants.SocketRushEventPresenceMemberRemoved,
				Payload:       _msg.ToJSON(),
				SocketID:      session.socketID,
				SocketIDs:     userSocketIds,
				SkipBroadcast: skipBroadcast,
			})
		} else {
			_ = storage.Manager.AdjustChannelCount(h.nodeID, channel, -1)
		}

		//channelCount := storage.Manager.GetChannelCount(channel)
		//if channelCount == 0 {
		//	// TODO: send webhook for channel vacated
		//	log.Logger().Traceln("Channel is empty; sending webhook")
		//	_ = storage.Manager.RemoveChannel(channel)
		//	event := pusherClient.WebhookEvent{
		//		Name:    string(constants.WebHookChannelVacated),
		//		Channel: string(channel),
		//	}
		//	dispatcher.Dispatcher.Dispatch(event)
		//	dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channel), dispatcher.Disconnect, event)
		//}

		// check if channel is empty, delete from list
		if len(h.localChannels[channel].Connections) == 0 {
			delete(h.localChannels, channel)
		}
	}
}

func (h *Hub) handleIncomingChannelEvent(message pubsub.ServerMessage) {
	var event ChannelEvent
	err := json.Unmarshal(message.Payload, &event)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling channel event: %v", err)
		return
	}
	if message.SkipBroadcast {
		return
	}
	message.SocketIDs = append(message.SocketIDs, message.SocketID)

	_ = h.publishChannelEventLocally(event, message.SocketIDs...)
}

func (h *Hub) publishChannelEventLocally(event ChannelEvent, socketIDsToExclude ...constants.SocketID) error {
	if event.Channel == "" {
		return errors.New("channel name is required")
	}

	// Check if the channel is a cache channel. If it is, update the cache with the event - even if there is no one subscribed
	if util.IsCacheChannel(event.Channel) {
		channelEventsToIgnore := []string{
			constants.PusherInternalPresenceMemberAdded,
			constants.PusherInternalPresenceMemberRemoved,
		}

		if !funk.ContainsString(channelEventsToIgnore, event.Event) && !util.IsClientEvent(event.Event) {
			log.Logger().Debug("Updating cache channel with event")
			eventPayload, err := json.Marshal(event)
			if err != nil {
				log.Logger().Errorf("Error marshalling cache event to JSON: %v", err)
			}
			cache.ChannelCache.SetEx(string(event.Channel), string(eventPayload), 30*time.Minute)
		}
	}

	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.localChannels[event.Channel] == nil {
		// channel not present on local node
		return nil
	}
	e := event.ToJSON()

	for socketID := range h.localChannels[event.Channel].Connections {
		if session, ok := h.sessions[socketID]; ok {
			// check if the socketID is in the socketIDsToExclude list
			if ok := util.ListContains(socketIDsToExclude, socketID); ok {
				continue
			}

			// check if the socketID is set and is the same as this event's socketID; if so, skip sending
			if session.socketID != event.SocketID {
				session.Send(e)
			}
		}
	}
	return nil
}

func (h *Hub) PublishChannelEventGlobally(event ChannelEvent) error {
	serverMessage := pubsub.ServerMessage{
		NodeID:  h.nodeID,
		Event:   constants.SocketRushEventChannelEvent,
		Payload: event.ToJSON(),
	}
	h.sendToOtherServers(serverMessage)
	return nil
}

func (h *Hub) IsAcceptingConnections() bool {
	return h.acceptingNewConnections
}

func (h *Hub) HandleClosure() {
	log.Logger().Info("Hub is closing, cleaning up all sessions")
	h.acceptingNewConnections = false

	var wg sync.WaitGroup
	wg.Add(len(h.sessions))
	for _, session := range h.sessions {
		go func() {
			session.CloseSession()
			wg.Done()
		}()
	}
	wg.Wait()
	close(h.stopChan)
}
