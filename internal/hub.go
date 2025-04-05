package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/env"
	"pusher/internal/clients"
	"pusher/internal/pubsub"
	"pusher/internal/storage"
	"pusher/internal/util"

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

type Hub struct {
	nodeID constants.NodeID

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
		nodeID:                  constants.NodeID(fmt.Sprintf("%s%s", newUuid[0:4], newUuid[len(newUuid)-4:])),
		localChannels:           make(map[constants.ChannelName]*Channel),
		sessions:                make(map[constants.SocketID]*Session),
		broadcast:               make(chan []byte),
		register:                make(chan *Session),
		unregister:              make(chan *Session),
		server2server:           make(chan pubsub.ServerMessage),
		cleanerPromotionChannel: make(chan pubsub.ServerMessage),
	}

	pubSubManagerDriver := env.GetString("PUBSUB_MANAGER", "local")
	storageManagerDriver := env.GetString("STORAGE_MANAGER", "local")

	var redisClient redis.UniversalClient

	if pubSubManagerDriver == "redis" || storageManagerDriver == "redis" {
		rc := &clients.RedisClient{}
		rErr := rc.InitRedis(env.GetString("REDIS_PREFIX", "pusher"))
		if rErr != nil {
			log.Logger().Fatal(rErr)
		}
		redisClient = rc.GetClient()
	}

	// Initialize the global PubSub Manager, used for broadcasting messages to all nodes
	hub.initializePubSubManager(pubSubManagerDriver, redisClient)

	// Initialize the global Storage Manager, used for storing channel counts and presence data
	hub.initializeStorageManager(storageManagerDriver, redisClient)

	GlobalHub = hub

	// Register the node with the storage manager
	nErr := storage.Manager.AddNewNode(hub.nodeID)
	if nErr != nil {
		log.Logger().Fatal(nErr)
	}

	// Start the background cleaner process
	go hub.startCleaner()

	return hub
}

func (h *Hub) initializeStorageManager(storageManagerDriver string, redisClient redis.UniversalClient) {
	switch storageManagerDriver {
	case "redis":
		storage.Manager = &storage.RedisStorage{
			Client:    redisClient,
			KeyPrefix: env.GetString("REDIS_PREFIX", "pusher"),
		}
	case "local":
		mgr := &storage.StandaloneStorageManager{}
		_ = mgr.Init()
		go mgr.ListenForMessages()
		storage.Manager = mgr

	default:
		log.Logger().Fatal("Unsupported storage manager")
	}
}

func (h *Hub) initializePubSubManager(pubSubManagerDriver string, redisClient redis.UniversalClient) {
	switch pubSubManagerDriver {
	case "redis":
		rPSM := &pubsub.RedisPubSubManager{Client: redisClient}
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

// startCleaner runs the background process to periodically clean up the globalChannels, presenceChannels, and localChannels maps from stale nodes
func (h *Hub) startCleaner() {
	interval := env.GetSeconds("CLEANER_INTERVAL", 60)
	log.Logger().Infof("Starting the cleaner - running every %d seconds", interval)
	_ = pubsub.PubSubManager.Publish(constants.SoketRushInternalChannel, pubsub.ServerMessage{
		NodeID:  GlobalHub.nodeID,
		Event:   constants.SocketRushEventCleanerPromote,
		Payload: []byte{},
	})

	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		log.Logger().Infoln("Stopping the cleaner")
	}()
	for {
		select {
		case _, ok := <-h.cleanerPromotionChannel:
			if !ok {
				return
			}
		case <-ticker.C:
			storage.Manager.Cleanup()
		}
	}
}

func (h *Hub) Run() {
	log.Logger().Infoln("Starting the hub")
	// Start the heartbeat broadcast - this sends out its own heartbeat and checks for other nodes
	go h.broadcastHeartbeat()

	// Subscribe to the pubsub channel for server-to-server messages
	go pubsub.PubSubManager.Subscribe(constants.SoketRushInternalChannel, h.server2server) // route incoming messages from pubsub to the server2server channel

	// start listening for hub activity
	for {
		select {
		case msg := <-h.server2server:
			h.handleServerToServerIncoming(msg)
		case session := <-h.register:
			h.mutex.Lock()
			h.sessions[session.socketID] = session
			h.mutex.Unlock()
		case session := <-h.unregister:
			h.mutex.Lock()
			delete(h.sessions, session.socketID)
			h.mutex.Unlock()
			// TODO: replace with cleanSession() logic
		}
	}
}

// function to send heartbeat to other nodes
func (h *Hub) broadcastHeartbeat() {
	log.Logger().Infof("Starting the node heartbeat broadcast; will broadcast every %d seconds", int(storage.NodePingInterval.Seconds()))
	ticker := time.NewTicker(storage.NodePingInterval)
	defer func() {
		ticker.Stop()
		log.Logger().Infoln("Stopping the node heartbeat broadcast")
	}()
	for {
		select {
		case <-ticker.C:
			// Broadcast to others that we're still alive
			storage.Manager.SendNodeHeartbeat(h.nodeID)
		}
	}
}

// sendToOtherServers sends a ServerMessage to all other nodes in the cluster via pubsub
func (h *Hub) sendToOtherServers(msg pubsub.ServerMessage) {
	err := pubsub.PubSubManager.Publish(constants.SoketRushInternalChannel, msg)
	if err != nil {
		log.Logger().Errorf("Failed to publish message to other servers: %v", err)
	}
}

// handleServerToServerIncoming handles messages received from other nodes in the cluster
// mostly used to broadcast out to connected clients
func (h *Hub) handleServerToServerIncoming(msg pubsub.ServerMessage) {
	switch msg.Event {
	case constants.SocketRushEventCleanerPromote:
		if h.cleanerPromotionChannel != nil {
			close(h.cleanerPromotionChannel)
			h.cleanerPromotionChannel = nil
		}

	case constants.SocketRushEventChannelEvent:
		h.handleIncomingChannelEvent(msg)
	}
}

// TODO: need to send the disconnected message about clients that were on that node (for presence)
//func (h *Hub) removeNode(nodeID constants.NodeID) {
//	h.mutex.Lock()
//	defer h.mutex.Unlock()
//
//	if _, ok := h.otherNodes[nodeID]; ok {
//		delete(h.otherNodes, nodeID)
//	}
//
//	// loop through channels and remove any node-specific entries
//	for _, nodes := range h.globalChannels {
//		if _, ok := nodes[constants.NodeID(nodeID)]; ok {
//			delete(nodes, constants.NodeID(nodeID))
//		}
//	}
//	for _, nodes := range h.presenceChannels {
//		if _, ok := nodes[constants.NodeID(nodeID)]; ok {
//			delete(nodes, constants.NodeID(nodeID))
//		}
//	}
//}

// addPresenceUser adds the socket to the local connections list, adds the info to redis, and notifies other nodes
func (h *Hub) addPresenceUser(socketID constants.SocketID, channel Channel, memberData pusherClient.MemberData) {
	log.Logger().Tracef("[%s]  Adding presence user to hub\n", socketID)

	localChannel := h.getOrCreateLocalChannel(channel.Name)

	// add the socket to the local channel
	localChannel.addSocketID(socketID)

	// prepare member data for storage and broadcast
	md, mdErr := json.Marshal(memberData)
	if mdErr != nil {
		log.Logger().Errorf("Error marshalling member data to send to other servers: %s", mdErr)
		return
	}

	// add the user to the pubsub connection
	_ = storage.Manager.AddUserToPresence(h.nodeID, channel.Name, socketID, memberData)

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

// handleRemovePresenceUser accepts a server event payload and removes the user from the presence channel
//func (h *Hub) handleRemovePresenceUser(serverMessage pubsub.ServerMessage) {
//	log.Logger().Traceln("Removing presence user")
//	var channelEvent ChannelEvent
//	var memberRemovedData MemberRemovedData
//	e := json.Unmarshal(serverMessage.Payload, &channelEvent)
//	if e != nil {
//		log.Logger().Errorf("Error unmarshalling server event channelEvent: %v", e)
//		return
//	}
//
//	if channelEvent.Channel == "" || channelEvent.Event != constants.PusherInternalPresenceMemberRemoved {
//		return
//	}
//
//	_ = h.publishChannelEventLocally(channelEvent)
//
//	e = json.Unmarshal([]byte(channelEvent.Data), &memberRemovedData)
//	if e != nil {
//		log.Logger().Errorf("Error unmarshalling member removed data: %v", e)
//		return
//	}
//
//	h.mutex.Lock()
//	defer h.mutex.Unlock()
//
//	// remove the user from the presence channel data on the hub
//	// if the channel is empty, remove the channel from the presenceChannels and announce the channel vacated
//	if h.presenceChannels[channelEvent.Channel] == nil {
//		return
//	}
//	if h.presenceChannels[channelEvent.Channel][serverMessage.NodeID] == nil {
//		return
//	}
//
//	// Check if the message originated from this node
//	//if serverMessage.NodeID == h.nodeID {
//	//	// inform connected sessions that the user has left
//	//	for socketId := range h.localChannels[channelEvent.Channel].Connections {
//	//		if socketId == serverMessage.SocketID {
//	//			continue
//	//		}
//	//		if session, ok := h.sessions[socketId]; ok {
//	//			session.Send(channelEvent.ToJSON())
//	//		}
//	//	}
//	//}
//	delete(h.presenceChannels[channelEvent.Channel][serverMessage.NodeID], serverMessage.SocketID)
//	if len(h.presenceChannels[channelEvent.Channel][serverMessage.NodeID]) == 0 {
//		delete(h.presenceChannels[channelEvent.Channel], serverMessage.NodeID)
//	}
//	if len(h.presenceChannels[channelEvent.Channel]) == 0 {
//		delete(h.presenceChannels, channelEvent.Channel)
//		// TODO: announce and send webhook for channel vacated
//	}
//}

func (h *Hub) getOrCreateLocalChannel(channel constants.ChannelName) *Channel {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.localChannels[channel] == nil {
		var t constants.ChannelType
		if util.IsPrivateChannel(channel) {
			t = constants.ChannelTypePrivate
		} else if util.IsPresenceChannel(channel) {
			t = constants.ChannelTypePresence
		} else if util.IsPrivateEncryptedChannel(channel) {
			t = constants.ChannelTypePrivateEncrypted
		} else {
			t = constants.ChannelTypePublic
		}
		newChannel := &Channel{
			Name:        channel,
			Connections: make(map[constants.SocketID]bool),
			Type:        t,
		}
		h.localChannels[channel] = newChannel
	}
	return h.localChannels[channel]
}

// addSessionToChannel adds a session to a channel on the hub for use when broadcasting messages
// also need to add the session to the pubsub channel count
func (h *Hub) addSessionToChannel(session *Session, channel *Channel) {
	h.getOrCreateLocalChannel(channel.Name).addSocketID(session.socketID)
	_ = storage.Manager.AddChannel(channel.Name)
	if channel.Type != constants.ChannelTypePresence {
		_ = storage.Manager.AdjustChannelCount(h.nodeID, channel.Name, 1)
	}
}

// removeSessionFromChannel removes a session from a channel on the hub, and also removes the session from the pubsub channel list.
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
		//log.Logger().Tracef("... ... ... removing session from channel: %s", channel)
		if h.localChannels[channel] != nil {
			delete(h.localChannels[channel].Connections, session.socketID)
		}

		if util.IsPresenceChannel(channel) {
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

			_ = storage.Manager.RemoveUserFromPresence(h.nodeID, channel, session.socketID)
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

		channelCount := storage.Manager.GetChannelCount(channel)

		if channelCount == 0 {
			// TODO: send webhook for channel vacated
			log.Logger().Traceln("Channel is empty; sending webhook")
			_ = storage.Manager.RemoveChannel(channel)
		}

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
