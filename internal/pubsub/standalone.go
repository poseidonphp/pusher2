package pubsub

import (
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/log"
)

type StandAlonePubSubManager struct {
	PubSubCore
	ch chan ServerMessage

	// globalChannels includes all channels that are not presence channels, showing the count of subscribers
	globalChannels map[constants.ChannelName]int64

	// presenceChannels stores presenceData for each user in a channel, organized by channel -> socket
	presenceChannels map[constants.ChannelName]map[constants.SocketID]pusherClient.MemberData

	nodeList map[constants.NodeID]bool

	// mutex for presenceChannels
	//mutex sync.RWMutex
}

func (s *StandAlonePubSubManager) Init() error {
	s.ch = make(chan ServerMessage)
	s.globalChannels = make(map[constants.ChannelName]int64)
	s.presenceChannels = make(map[constants.ChannelName]map[constants.SocketID]pusherClient.MemberData)
	s.nodeList = make(map[constants.NodeID]bool)

	log.Logger().Traceln("Initialized StandAlonePubSubManager")
	return nil
}

func (s *StandAlonePubSubManager) Subscribe(_ constants.ChannelName, receiveChannel chan<- ServerMessage) {
	for {
		select {
		case msg := <-s.ch:
			receiveChannel <- msg
		}
	}
}

func (s *StandAlonePubSubManager) Publish(_ constants.ChannelName, msg ServerMessage) error {
	s.ch <- msg
	return nil
}

//func (s *StandAlonePubSubManager) AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error {
//	s.mutex.Lock()
//	defer s.mutex.Unlock()
//	if _, ok := s.presenceChannels[channelName]; !ok {
//		s.presenceChannels[channelName] = make(map[constants.SocketID]pusherClient.MemberData)
//	}
//	s.presenceChannels[channelName][socketID] = memberData
//	return nil
//}
//
//func (s *StandAlonePubSubManager) RemoveUserFromPresence(_ constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error {
//	return nil
//}
//
//func (s *StandAlonePubSubManager) AdjustChannelCount(_ constants.NodeID, channelName constants.ChannelName, countToAdd int64) error {
//	s.mutex.Lock()
//	defer s.mutex.Unlock()
//	if _, ok := s.globalChannels[channelName]; !ok {
//		s.globalChannels[channelName] = 0
//	}
//	s.globalChannels[channelName] += countToAdd
//	return nil
//}
//
//func (s *StandAlonePubSubManager) GetPresenceData(channelName constants.ChannelName) ([]byte, error) {
//	_presenceData := payloads.PresenceData{
//		IDs:   []string{},
//		Hash:  map[string]map[string]string{},
//		Count: 0,
//	}
//
//	s.mutex.Lock()
//	for _, mData := range s.presenceChannels[channelName] {
//		_presenceData.Hash[mData.UserID] = mData.UserInfo
//		_presenceData.IDs = append(_presenceData.IDs, mData.UserID)
//	}
//	s.mutex.Unlock()
//	//if newlyAddedMember != nil {
//	//	_presenceData.Hash[newlyAddedMember.UserID] = newlyAddedMember.UserInfo
//	//	_presenceData.IDs = append(_presenceData.IDs, newlyAddedMember.UserID)
//	//}
//	_presenceData.Count = len(_presenceData.IDs)
//
//	presenceData, pErr := json.Marshal(map[string]payloads.PresenceData{"presence": _presenceData})
//	if pErr != nil {
//		log.Logger().Errorf("Error marshalling presence data: %s", pErr)
//		return nil, pErr
//	}
//	return presenceData, nil
//}
//
//func (s *StandAlonePubSubManager) GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error) {
//	return nil, nil
//}
//
//func (s *StandAlonePubSubManager) GetChannelCount(channelName constants.ChannelName) int64 {
//	s.mutex.Lock()
//	defer s.mutex.Unlock()
//	if util.IsPresenceChannel(channelName) {
//		if count, ok := s.presenceChannels[channelName]; ok {
//			return int64(len(count))
//		}
//		return 0
//	}
//	if count, ok := s.globalChannels[channelName]; ok {
//		return count
//	}
//	return 0
//}
