package storage

import (
	"context"
	"encoding/json"
	"errors"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/pubsub"
	"pusher/internal/util"
	"pusher/log"
	"sync"
	"time"
)

const commChannelName = "pusher:localstorage"

type StandaloneStorageManager struct {
	listOfNodes      map[constants.NodeID]int64
	presenceChannels map[constants.ChannelName]map[constants.NodeID]map[constants.SocketID]pusherClient.MemberData
	channelCounts    map[constants.ChannelName]map[constants.NodeID]int64
	mutex            sync.RWMutex
	commChannel      chan pubsub.ServerMessage
	Pubsub           pubsub.PubSubManagerContract
}

// *********** NON-INTERFAFCE METHODS ***********

func (s *StandaloneStorageManager) listenForMessages() {
	if s.Pubsub == nil {
		log.Logger().Error("PubSubManager is not initialized")
		return
	}
	defer func() {
		log.Logger().Warn("Exiting local storage message listener")
	}()
	log.Logger().Traceln("Listening for messages on local storage channel")
	subReady := make(chan struct{})
	go s.Pubsub.SubscribeWithNotify(context.Background(), commChannelName, s.commChannel, subReady)
	<-subReady
	for {
		select {
		case msg := <-s.commChannel:
			switch msg.Event {
			case ServerEventNewNodeJoined:
				_ = s.AddNewNode(msg.NodeID)
			case ServerEventNodeLeft:
				_ = s.RemoveNode(msg.NodeID)
			case ServerHeartbeat:
				s.updateNodeHeartbeat(msg.NodeID)
			default:
				log.Logger().Tracef("Received unknown event in storageLocal: %s", msg.Event)
			}
		}
	}
}

func (s *StandaloneStorageManager) updateNodeHeartbeat(nodeID constants.NodeID) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	log.Logger().Tracef("Updating heartbeat for node %s", nodeID)
	if _, ok := s.listOfNodes[nodeID]; ok {
		s.listOfNodes[nodeID] = time.Now().Unix()
	}
}

// *********** INTERFACE-SPECIFIC METHODS ***********

// Init initializes the StandaloneStorageManager
func (s *StandaloneStorageManager) Init() error {
	s.listOfNodes = make(map[constants.NodeID]int64)
	s.presenceChannels = make(map[constants.ChannelName]map[constants.NodeID]map[constants.SocketID]pusherClient.MemberData)
	s.channelCounts = make(map[constants.ChannelName]map[constants.NodeID]int64)
	s.commChannel = make(chan pubsub.ServerMessage)
	if s.Pubsub == nil {
		log.Logger().Error("PubSubManager is not initialized")
		return errors.New("PubSubManager is not initialized in StandaloneStorageManager")
	}
	return nil
}

// Start the storage manager
func (s *StandaloneStorageManager) Start() {
	//// Use a WaitGroup to track goroutines
	//var wg sync.WaitGroup
	//wg.Add(1)

	//go func() {
	//defer wg.Done()
	//s.listenForMessages()
	//}()

	go s.listenForMessages()
}

func (s *StandaloneStorageManager) AddNewNode(nodeID constants.NodeID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// check if node already exists
	if _, ok := s.listOfNodes[nodeID]; ok {
		return nil
	}
	s.listOfNodes[nodeID] = time.Now().Unix()
	return nil
}

func (s *StandaloneStorageManager) RemoveNode(nodeID constants.NodeID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// check if node already exists
	if _, ok := s.listOfNodes[nodeID]; ok {
		// TODO: clear all channel data for this node
		delete(s.listOfNodes, nodeID)
	}
	return nil
}

// PurgeNodeData removes a node from the list of nodes, and clears any channel data associated with the node.
// This process can be time-consuming if there are many channels (ie one per user with a lot of users).
func (s *StandaloneStorageManager) PurgeNodeData(nodeToPurge constants.NodeID) error {
	// first remove any channel counts for this node
	s.mutex.Lock()

nodeCountsLoop:
	for _, nodeCounts := range s.channelCounts {
		for nodeID, _ := range nodeCounts {
			if nodeID == nodeToPurge {
				delete(nodeCounts, nodeID)
				break nodeCountsLoop
			}
		}
	}

	// now remove any presence data for this node
nodeDataLoop:
	for _, nodeData := range s.presenceChannels {
		for nodeID, _ := range nodeData {
			if nodeID == nodeToPurge {
				delete(nodeData, nodeID)
				break nodeDataLoop
			}
		}
	}
	s.mutex.Unlock()
	return s.RemoveNode(nodeToPurge)
}

func (s *StandaloneStorageManager) GetAllNodes() ([]string, error) {
	nodeList := make([]string, 0)

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for nodeID := range s.listOfNodes {
		if nodeID == "" {
			continue
		}
		nodeList = append(nodeList, string(nodeID))
	}
	return nodeList, nil
}

func (s *StandaloneStorageManager) SendNodeHeartbeat(nodeID constants.NodeID) *time.Time {
	_msg := pubsub.ServerMessage{
		NodeID:  nodeID,
		Event:   ServerHeartbeat,
		Payload: []byte{},
	}
	log.Logger().Tracef("Sending heartbeat for node %s", nodeID)
	currentTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pErr := s.Pubsub.Publish(ctx, commChannelName, _msg)
	if pErr != nil {
		log.Logger().Errorf("Error publishing heartbeat message: %v", pErr)
		return nil
	}
	log.Logger().Tracef("...Sent heartbeat for node %s", nodeID)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for nID, lastHeartbeat := range s.listOfNodes {
		if time.Now().Unix()-lastHeartbeat > int64(NodePingInterval.Seconds())+5 {
			log.Logger().Warnf("Node %s has not sent a heartbeat in over %f seconds, removing it from the list of nodes", nID, NodePingInterval.Seconds()+5)
			_ = s.RemoveNode(nID)
		}
	}
	return &currentTime
}

func (s *StandaloneStorageManager) AddChannel(channel constants.ChannelName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.channelCounts[channel]; !ok {
		s.channelCounts[channel] = make(map[constants.NodeID]int64)
	}
	return nil
}

func (s *StandaloneStorageManager) RemoveChannel(channel constants.ChannelName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.channelCounts[channel]; ok {
		delete(s.channelCounts, channel)
	}
	return nil
}

func (s *StandaloneStorageManager) Channels() []constants.ChannelName {
	channelList := make([]constants.ChannelName, len(s.channelCounts)+len(s.presenceChannels))

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for channel := range s.channelCounts {
		channelList = append(channelList, channel)
	}

	for channel := range s.presenceChannels {
		channelList = append(channelList, channel)
	}

	return channelList
}

func (s *StandaloneStorageManager) AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) (newCount int64, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.channelCounts[channelName]; !ok {
		s.channelCounts[channelName][nodeID] = 0
	}
	s.channelCounts[channelName][nodeID] += countToAdd

	newCount = s.channelCounts[channelName][nodeID]

	//s.handleChannelCountChanges(channelName, newCount, countToAdd)
	return
}

// GetChannelCount returns the count of subscribers for a channel (not unique to user ids)
func (s *StandaloneStorageManager) GetChannelCount(channelName constants.ChannelName) int64 {
	runningCount := int64(0)

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if util.IsPresenceChannel(channelName) {
		if _, ok := s.presenceChannels[channelName]; !ok {
			return 0
		}

		for _, nodeData := range s.presenceChannels[channelName] {
			runningCount += int64(len(nodeData))
		}
	} else {
		for _, count := range s.channelCounts[channelName] {
			runningCount += count
		}
	}
	return runningCount
}

func (s *StandaloneStorageManager) AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.presenceChannels[channelName]; !ok {
		s.presenceChannels[channelName] = make(map[constants.NodeID]map[constants.SocketID]pusherClient.MemberData)
	}
	if _, ok := s.presenceChannels[channelName][nodeID]; !ok {
		s.presenceChannels[channelName][nodeID] = make(map[constants.SocketID]pusherClient.MemberData)
	}

	s.presenceChannels[channelName][nodeID][socketID] = memberData

	return nil
}

func (s *StandaloneStorageManager) RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	//var newCount int64
	if _, ok := s.presenceChannels[channelName]; ok {
		if _, ok := s.presenceChannels[channelName][nodeID]; ok {
			delete(s.presenceChannels[channelName][nodeID], socketID)
			//newCount = int64(len(s.presenceChannels[channelName][nodeID]))
		}
	}
	return nil
}

// GetPresenceData returns the presence data for a given channel and socketIDs for the current user (list of users and their data).
func (s *StandaloneStorageManager) GetPresenceData(channelName constants.ChannelName, currentUser pusherClient.MemberData) ([]byte, []constants.SocketID, error) {
	_presenceData := payloads.PresenceData{
		Count: 0,
		Hash:  map[string]map[string]string{},
		IDs:   []string{},
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	usersSocketIDs := make([]constants.SocketID, 0)

	// append the current user, since they are likely not in the list yet
	if currentUser.UserID != "" {
		_presenceData.Hash[currentUser.UserID] = currentUser.UserInfo
		_presenceData.IDs = append(_presenceData.IDs, currentUser.UserID)
	}

	for _, nodeData := range s.presenceChannels[channelName] {
		for socketId, memberData := range nodeData {
			if memberData.UserID == currentUser.UserID {
				usersSocketIDs = append(usersSocketIDs, socketId)
			}
			_presenceData.Hash[memberData.UserID] = memberData.UserInfo
			_presenceData.IDs = append(_presenceData.IDs, memberData.UserID)
		}
	}
	_presenceData.Count = len(_presenceData.IDs)
	presenceData, pErr := json.Marshal(map[string]payloads.PresenceData{"presence": _presenceData})
	if pErr != nil {
		return nil, usersSocketIDs, pErr
	}
	return presenceData, usersSocketIDs, nil
}

func (s *StandaloneStorageManager) GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if _, ok := s.presenceChannels[channelName]; ok {
		if _, ok := s.presenceChannels[channelName][nodeID]; ok {
			if memberData, ok := s.presenceChannels[channelName][nodeID][socketID]; ok {
				return &memberData, nil
			}
		}
	}
	return nil, errors.New("socket not found")
}

func (s *StandaloneStorageManager) SocketDidHeartbeat(_ constants.NodeID, _ constants.SocketID, _ map[constants.ChannelName]constants.ChannelName) error {
	return nil
}

func (s *StandaloneStorageManager) Cleanup() {
	return
}
