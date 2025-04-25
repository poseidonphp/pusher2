package internal

//
// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"sync/atomic"
//
// 	"github.com/google/uuid"
// 	pusherClient "github.com/pusher/pusher-http-go/v5"
// 	"github.com/thoas/go-funk"
// 	"pusher/internal/config"
// 	"pusher/internal/pubsub"
// 	"pusher/internal/queues"
// 	"pusher/internal/storage"
// 	"pusher/internal/util"
//
// 	"sync"
// 	"time"
//
// 	"pusher/internal/constants"
// 	"pusher/log"
// )
//
// var GlobalHub *Hub
//
// var LastHeartbeat atomic.Value
//
// type Hub struct {
// 	nodeID                  constants.NodeID
// 	acceptingNewConnections bool
// 	ctx                     context.Context
// 	cancel                  context.CancelFunc
// 	config                  *config.ServerConfig
// 	localChannels           map[constants.ChannelName]*Channel // stores the sockets that are subscribed to channels on this node
// 	sessions                map[constants.SocketID]*Session    // store all sessions connected to this instance
// 	broadcast               chan []byte
// 	register                chan *Session // Channel to handle connecting new sessions
// 	unregister              chan *Session // Channel to handle disconnecting sessions
// 	server2server           chan pubsub.ServerMessage
// 	mutex                   sync.Mutex
// 	pubsubManager           *pubsub.PubSubManagerContract
// 	cleanerPromotionChannel chan pubsub.ServerMessage
// 	// stopChan                chan struct{}
// }
//
// // MessageToClient is a struct to send messages to the client - TESTING instead of using individual go routines on each session
// type MessageToClient struct {
// 	Session *Session
// 	Message []byte
// }
//
// func NewHub(ctx context.Context, serverConfig *config.ServerConfig) *Hub {
// 	newUuid := uuid.New().String()
// 	hubCtx, hubCancel := context.WithCancel(ctx)
// 	hub := &Hub{
// 		ctx:                     hubCtx,
// 		cancel:                  hubCancel,
// 		config:                  serverConfig,
// 		acceptingNewConnections: true,
// 		nodeID:                  constants.NodeID(fmt.Sprintf("%s%s", newUuid[0:4], newUuid[len(newUuid)-4:])),
// 		localChannels:           make(map[constants.ChannelName]*Channel),
// 		sessions:                make(map[constants.SocketID]*Session),
// 		broadcast:               make(chan []byte),
// 		register:                make(chan *Session, 100),
// 		unregister:              make(chan *Session, 100),
// 		server2server:           make(chan pubsub.ServerMessage),
// 		cleanerPromotionChannel: make(chan pubsub.ServerMessage),
// 	}
//
// 	GlobalHub = hub
//
// 	return hub
// }
//
// // Run starts the hub. This is the core function for managing all websocket connections
// func (h *Hub) Run() {
// 	log.Logger().Infoln("Starting the hub")
//
// 	// Register the node with the storage manager
// 	nErr := h.config.StorageManager.AddNewNode(h.nodeID)
// 	if nErr != nil {
// 		log.Logger().Fatal(nErr)
// 	}
//
// 	// Subscribe to the pubsub Channel for server-to-server messages
// 	subReady := make(chan struct{})
// 	go h.config.PubSubManager.SubscribeWithNotify(h.ctx, constants.SocketRushInternalChannel, h.server2server, subReady) // route incoming messages from pubsub to the server2server Channel
// 	<-subReady                                                                                                           // wait for the subscription to complete
//
// 	// Start the storage manager
// 	go h.config.StorageManager.Start()
//
// 	// Start the background cleaner process
// 	// time.Sleep(1 * time.Second) // wait for the storage manager to start
//
// 	// // Start the heartbeat broadcast - this sends out its own heartbeat and checks for other nodes
// 	// go h.broadcastHeartbeat()
// 	// go h.monitorHeartbeat()
// 	//
// 	// // Star the cleaner process for removing stale nodes
// 	// go h.startCleaner()
//
// 	// start listening for hub activity
// 	for {
// 		select {
// 		case msg := <-h.server2server:
// 			h.handleServerToServerIncoming(msg)
// 		case session := <-h.register:
// 			log.Logger().Tracef("[%s]  Hub: Registering new socket id: %s\n", session.socketID, session.socketID)
// 			h.mutex.Lock()
// 			h.sessions[session.socketID] = session
// 			h.mutex.Unlock()
// 			log.Logger().Tracef("[%s]  Hub: Registered new socket id: %s\n", session.socketID, session.socketID)
// 		case session := <-h.unregister:
// 			h.mutex.Lock()
// 			delete(h.sessions, session.socketID)
// 			h.mutex.Unlock()
// 			// TODO: replace with cleanSession() logic
// 		case <-h.ctx.Done():
// 			log.Logger().Infoln("... Stopping the hub due to shutdown signal")
// 			time.Sleep(1 * time.Second)
// 			return
// 		}
// 	}
// }
//
// // startCleaner runs the background process to periodically clean up the globalChannels, presenceChannels, and localChannels maps from stale nodes
// // func (h *Hub) startCleaner() {
// // 	interval := h.config.HubCleanerInterval
// // 	gracefulExit := false
// //
// // 	log.Logger().Infof("Starting the cleaner - running every %d seconds", int64(interval.Seconds()))
// //
// // 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// // 	defer cancel()
// //
// // 	pErr := h.config.PubSubManager.Publish(ctx, constants.SocketRushInternalChannel, pubsub.ServerMessage{
// // 		NodeID:  GlobalHub.nodeID,
// // 		Event:   constants.SocketRushEventCleanerPromote,
// // 		Payload: []byte{},
// // 	})
// // 	if pErr != nil {
// // 		log.Logger().Errorf("Error publishing cleaner promotion message: %v", pErr)
// // 		return
// // 	}
// //
// // 	ticker := time.NewTicker(interval)
// // 	defer func() {
// // 		ticker.Stop()
// // 		if !gracefulExit {
// // 			log.Logger().Warnln("Stopping the cleaner")
// // 		}
// //
// // 	}()
// // 	for {
// // 		select {
// // 		case _, ok := <-h.cleanerPromotionChannel:
// // 			if !ok {
// // 				gracefulExit = true
// // 				return
// // 			}
// // 		case <-ticker.C:
// // 			h.config.StorageManager.Cleanup()
// // 		case <-h.ctx.Done():
// // 			log.Logger().Infoln("... Stopping the cleaner due to shutdown signal")
// // 			gracefulExit = true
// // 			return
// // 		}
// // 	}
// // }
//
// // function to send heartbeat to other nodes
// func (h *Hub) broadcastHeartbeat() {
// 	log.Logger().Infof("Starting the node heartbeat broadcast; will broadcast every %d seconds", int(storage.NodePingInterval.Seconds()))
// 	ticker := time.NewTicker(storage.NodePingInterval)
// 	gracefulExit := false
//
// 	defer func() {
// 		if !gracefulExit {
// 			log.Logger().Warnln("Stopping the node heartbeat broadcast")
// 		}
//
// 		ticker.Stop()
// 	}()
//
// 	for {
// 		select {
// 		case <-ticker.C:
// 			// Broadcast to others that we're still alive
// 			func() {
// 				defer func() {
// 					if r := recover(); r != nil {
// 						log.Logger().Errorf("Recovered from panic in broadcastHeartbeat: %v", r)
// 					}
// 				}()
// 				t := h.config.StorageManager.SendNodeHeartbeat(h.nodeID)
// 				if t == nil {
// 					log.Logger().Errorf("Failed to send heartbeat to storage manager")
// 					return
// 				}
// 				LastHeartbeat.Store(*t)
// 			}()
// 		case <-h.ctx.Done():
// 			log.Logger().Infoln("... Stopping heartbeat broadcast due to shutdown signal")
// 			gracefulExit = true
// 			return
// 		}
// 	}
// }
//
// func (h *Hub) monitorHeartbeat() {
// 	log.Logger().Debug("Starting heartbeat monitor")
// 	ticker := time.NewTicker(15 * time.Second)
// 	defer ticker.Stop()
//
// 	for {
// 		select {
// 		case <-ticker.C:
// 			last := LastHeartbeat.Load().(time.Time)
// 			if time.Since(last) > 20*time.Second {
// 				log.Logger().Warn("Heartbeat appears to have stalled")
// 				// You could restart the heartbeat goroutine here if needed
// 			}
// 		case <-h.ctx.Done():
// 			log.Logger().Info("... Stopping heartbeat monitor due to shutdown signal")
// 			return
// 		}
// 	}
// }
//
// // sendToOtherServers sends a ServerMessage to all other nodes in the cluster via pubsub
// func (h *Hub) sendToOtherServers(msg pubsub.ServerMessage) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 	defer cancel()
// 	err := h.config.PubSubManager.Publish(ctx, constants.SocketRushInternalChannel, msg)
// 	if err != nil {
// 		log.Logger().Errorf("Failed to publish message to other servers: %v", err)
// 	}
// }
//
// // handleServerToServerIncoming handles messages received from other nodes in the cluster
// // mostly used to broadcast out to connected clients
// func (h *Hub) handleServerToServerIncoming(msg pubsub.ServerMessage) {
// 	switch msg.Event {
// 	case constants.SocketRushEventCleanerPromote:
// 		if h.cleanerPromotionChannel != nil && h.nodeID != msg.NodeID {
// 			close(h.cleanerPromotionChannel)
// 			h.cleanerPromotionChannel = nil
// 		}
//
// 	case constants.SocketRushEventChannelEvent:
// 		h.handleIncomingChannelEvent(msg)
// 	}
// }
//
// // addPresenceUser adds the socket to the local connections list, adds the info to redis, and notifies other nodes
// func (h *Hub) addPresenceUser(socketID constants.SocketID, channel *Channel, memberData pusherClient.MemberData) {
// 	log.Logger().Tracef("[%s]  Adding presence user to hub\n", socketID)
//
// 	localChannel := h.getOrCreateLocalChannel(channel)
//
// 	// add the socket to the local Channel
// 	localChannel.addSocketID(socketID)
//
// 	// prepare member data for storage and broadcast
// 	md, mdErr := json.Marshal(memberData)
// 	if mdErr != nil {
// 		log.Logger().Errorf("Error marshalling member data to send to other servers: %s", mdErr)
// 		return
// 	}
//
// 	// add the user to the pubsub connection
// 	err := h.config.StorageManager.AddUserToPresence(h.nodeID, channel.Name, socketID, memberData)
// 	if err != nil {
// 		log.Logger().Errorf("Error adding user to presence: %s", err)
// 		return
// 	}
// 	evt := pusherClient.WebhookEvent{
// 		Name:    string(constants.WebHookMemberAdded),
// 		Channel: string(channel.Name),
// 		UserID:  memberData.UserID,
// 	}
// 	bounceString := fmt.Sprintf("Channel:%s:%s", channel.Name, memberData.UserID)
//
// 	h.config.QueueManager.DispatchFlap(bounceString, queues.Connect, evt)
//
// 	userSocketIds := storage.GetPresenceSocketsForUserID(h.config.StorageManager, channel.Name, memberData.UserID)
// 	skipBroadcast := false
// 	if len(userSocketIds) > 1 {
// 		// if there are other sockets for this user, don't broadcast the message
// 		skipBroadcast = true
// 	}
//
// 	channelEvent := ChannelEvent{
// 		Event:   constants.PusherInternalPresenceMemberAdded,
// 		Channel: channel.Name,
// 		Data:    string(md),
// 	}
//
// 	s2s := pubsub.ServerMessage{
// 		NodeID:        GlobalHub.nodeID,
// 		Event:         constants.SocketRushEventChannelEvent,
// 		Payload:       channelEvent.ToJSON(),
// 		SocketID:      socketID,
// 		SocketIDs:     userSocketIds,
// 		SkipBroadcast: skipBroadcast,
// 	}
//
// 	// send to all nodes for broadcast to all clients
// 	h.sendToOtherServers(s2s)
// }
//
// // getOrCreateLocalChannel ensures the Channel exists in the localChannels map which is used for broadcasting to connected clients
// func (h *Hub) getOrCreateLocalChannel(channel *Channel) *Channel {
// 	h.mutex.Lock()
// 	defer h.mutex.Unlock()
//
// 	if h.localChannels[channel.Name] == nil {
// 		h.localChannels[channel.Name] = channel
// 	}
// 	return h.localChannels[channel.Name]
// }
//
// // addSessionToChannel adds a session to a Channel on the hub for use when broadcasting messages
// // also need to add the session to the pubsub Channel count
// func (h *Hub) addSessionToChannel(session *Session, channel *Channel) {
// 	h.getOrCreateLocalChannel(channel).addSocketID(session.socketID)
// 	err := h.config.StorageManager.AddChannel(channel.Name)
// 	if err != nil {
// 		log.Logger().Errorf("Error adding Channel to storage manager: %s", err)
// 	}
//
// 	if channel.Type != constants.ChannelTypePresence {
// 		newCount, countErr := h.config.StorageManager.AdjustChannelCount(h.nodeID, channel.Name, 1)
// 		if countErr != nil {
// 			log.Logger().Errorf("Error adjusting Channel count: %s", countErr)
// 		}
//
// 		// dispatch event
// 		h.config.QueueManager.SendChannelCountChanges(channel.Name, newCount, 1)
// 	}
// }
//
// // removeSessionFromChannels removes a session from a Channel on the hub, and also removes the session from the pubsub Channel list.
// // If a Channel is empty after removal, the webhook for vacated Channel is sent.
// // If no channels are provided, the session is removed from all channels.
// func (h *Hub) removeSessionFromChannels(session *Session, channels ...constants.ChannelName) {
// 	h.mutex.Lock()
// 	defer h.mutex.Unlock()
//
// 	var channelsToRemove []constants.ChannelName
//
// 	if len(channels) > 0 {
// 		channelsToRemove = channels
// 	} else {
// 		for channel := range session.subscriptions {
// 			session.Tracef("gracefully removing session from Channel: %s", channel)
// 			channelsToRemove = append(channelsToRemove, channel)
// 		}
// 	}
//
// 	for _, channel := range channelsToRemove {
// 		session.Tracef("... ... ... removing session from Channel: %s", channel)
// 		if h.localChannels[channel] != nil {
// 			delete(h.localChannels[channel].Connections, session.socketID)
// 		}
// 		channelObj := h.localChannels[channel]
//
// 		if channelObj.Type == constants.ChannelTypePresence {
// 			presenceData, gErr := h.config.StorageManager.GetPresenceDataForSocket(h.nodeID, channel, session.socketID)
// 			if gErr != nil {
// 				log.Logger().Errorf("Error getting presence data for socket: %v", gErr)
// 				continue
// 			}
// 			userSocketIds := storage.GetPresenceSocketsForUserID(h.config.StorageManager, channel, presenceData.UserID)
// 			skipBroadcast := false
// 			if len(userSocketIds) > 1 {
// 				// if there are other sockets for this user, don't broadcast the message
// 				skipBroadcast = true
// 			}
//
// 			rErr := h.config.StorageManager.RemoveUserFromPresence(h.nodeID, channel, session.socketID)
// 			if rErr != nil {
// 				log.Logger().Errorf("Error removing user from presence: %s", rErr)
// 				continue
// 			}
//
// 			evt := pusherClient.WebhookEvent{
// 				Name:    string(constants.WebHookMemberRemoved),
// 				Channel: string(channel),
// 				UserID:  presenceData.UserID,
// 			}
// 			bounceString := fmt.Sprintf("Channel:%s:%s", channel, presenceData.UserID)
// 			h.config.QueueManager.DispatchFlap(bounceString, queues.Connect, evt)
//
// 			// announce that the user has left the Channel
// 			mrd := &MemberRemovedData{UserID: presenceData.UserID}
// 			_msg := ChannelEvent{
// 				Event:   constants.PusherInternalPresenceMemberRemoved,
// 				Channel: channel,
// 				Data:    mrd.ToString(),
// 			}
// 			GlobalHub.sendToOtherServers(pubsub.ServerMessage{
// 				NodeID:        GlobalHub.nodeID,
// 				Event:         constants.SocketRushEventChannelEvent,
// 				Payload:       _msg.ToJSON(),
// 				SocketID:      session.socketID,
// 				SocketIDs:     userSocketIds,
// 				SkipBroadcast: skipBroadcast,
// 			})
// 		} else {
// 			newCount, err := h.config.StorageManager.AdjustChannelCount(h.nodeID, channel, -1)
// 			if err != nil {
// 				log.Logger().Errorf("Error adjusting Channel count: %s", err)
// 				continue
// 			}
// 			h.config.QueueManager.SendChannelCountChanges(channel, newCount, -1)
// 		}
//
// 		// channelCount := storage.Manager.GetChannelCount(Channel)
// 		// if channelCount == 0 {
// 		//	// TODO: send webhook for Channel vacated
// 		//	log.Logger().Traceln("Channel is empty; sending webhook")
// 		//	_ = storage.Manager.RemoveChannel(Channel)
// 		//	event := pusherClient.WebhookEvent{
// 		//		Name:    string(constants.WebHookChannelVacated),
// 		//		Channel: string(Channel),
// 		//	}
// 		//	dispatcher.Dispatcher.Dispatch(event)
// 		//	dispatcher.DispatchBuffer.HandleEvent("Channel:"+string(Channel), dispatcher.Disconnect, event)
// 		// }
//
// 		// check if Channel is empty, delete from list
// 		if len(h.localChannels[channel].Connections) == 0 {
// 			delete(h.localChannels, channel)
// 		}
// 	}
// }
//
// func (h *Hub) handleIncomingChannelEvent(message pubsub.ServerMessage) {
// 	var event ChannelEvent
// 	err := json.Unmarshal(message.Payload, &event)
// 	if err != nil {
// 		log.Logger().Errorf("Error unmarshalling Channel event: %v", err)
// 		return
// 	}
// 	if message.SkipBroadcast {
// 		return
// 	}
// 	message.SocketIDs = append(message.SocketIDs, message.SocketID)
//
// 	_ = h.publishChannelEventLocally(event, message.SocketIDs...)
// }
//
// func (h *Hub) publishChannelEventLocally(event ChannelEvent, socketIDsToExclude ...constants.SocketID) error {
// 	if event.Channel == "" {
// 		return errors.New("Channel name is required")
// 	}
//
// 	// Check if the Channel is a cache Channel. If it is, update the cache with the event - even if there is no one subscribed
// 	if util.IsCacheChannel(event.Channel) {
// 		channelEventsToIgnore := []string{
// 			constants.PusherInternalPresenceMemberAdded,
// 			constants.PusherInternalPresenceMemberRemoved,
// 		}
//
// 		if !funk.ContainsString(channelEventsToIgnore, event.Event) && !util.IsClientEvent(event.Event) {
// 			log.Logger().Debug("Updating cache Channel with event")
// 			eventPayload, err := json.Marshal(event)
// 			if err != nil {
// 				log.Logger().Errorf("Error marshalling cache event to JSON: %v", err)
// 			}
// 			h.config.ChannelCacheManager.SetEx(string(event.Channel), string(eventPayload), 30*time.Minute)
// 		}
// 	}
//
// 	h.mutex.Lock()
// 	defer h.mutex.Unlock()
// 	if h.localChannels[event.Channel] == nil {
// 		// Channel not present on local node
// 		return nil
// 	}
// 	e := event.ToJSON()
//
// 	for socketID := range h.localChannels[event.Channel].Connections {
// 		if session, ok := h.sessions[socketID]; ok {
// 			// check if the socketID is in the socketIDsToExclude list
// 			if ok = util.ListContains(socketIDsToExclude, socketID); ok {
// 				continue
// 			}
//
// 			// check if the socketID is set and is the same as this event's socketID; if so, skip sending
// 			if session.socketID != event.SocketID {
// 				session.Send(e)
// 			}
// 		}
// 	}
// 	return nil
// }
//
// func (h *Hub) PublishChannelEventGlobally(event ChannelEvent) error {
// 	serverMessage := pubsub.ServerMessage{
// 		NodeID:  h.nodeID,
// 		Event:   constants.SocketRushEventChannelEvent,
// 		Payload: event.ToJSON(),
// 	}
// 	h.sendToOtherServers(serverMessage)
// 	return nil
// }
//
// // ValidatePresenceChannelRequirements - Checks if the presence Channel requirements are met for a given request
// func (h *Hub) ValidatePresenceChannelRequirements(channel constants.ChannelName, userData string) (presenceMemberData pusherClient.MemberData, err *util.Error) {
// 	// if int64(len(storage.PresenceChannelUserIDs(h.config.StorageManager, Channel))) >= h.config.MaxPresenceUsers {
// 	// 	err = util.NewError(util.ErrCodeMaxPresenceSubscribers)
// 	// }
// 	//
// 	// if len(userData) > h.config.MaxPresenceUserDataBytes {
// 	// 	err = util.NewError(util.ErrCodePresenceUserDataTooMuch)
// 	// }
//
// 	uErr := json.Unmarshal([]byte(userData), &presenceMemberData)
// 	if uErr != nil {
// 		log.Logger().Errorf("Error unmarshalling presence Channel data: %s", uErr)
// 		err = util.NewError(util.ErrCodeInvalidPayload)
// 	}
// 	if len(presenceMemberData.UserID) > constants.MaxPresenceUserIDLength {
// 		err = util.NewError(util.ErrCodePresenceUserIDTooLong)
// 	}
//
// 	return
// }
//
// func (h *Hub) IsAcceptingConnections() bool {
// 	return h.acceptingNewConnections
// }
//
// func (h *Hub) HandleClosure() {
// 	log.Logger().Info("Hub is closing, cleaning up all sessions")
// 	h.acceptingNewConnections = false
//
// 	// Cancel the hub's context. This will signal all session goroutines to stop
// 	h.cancel()
// }
