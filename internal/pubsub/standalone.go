package pubsub

import (
	"pusher/log"
	"sync"
)

type localPubSubMessage struct {
	Message ServerMessage
	Channel string
}

type pubSubChannelConnection struct {
	ReceiveChannel  chan<- ServerMessage
	InternalChannel chan localPubSubMessage
}

type StandAlonePubSubManager struct {
	PubSubCore
	ch       chan localPubSubMessage
	Channels map[string]pubSubChannelConnection
	mutex    sync.Mutex
}

func (s *StandAlonePubSubManager) Init() error {
	s.ch = make(chan localPubSubMessage)
	s.Channels = make(map[string]pubSubChannelConnection)
	log.Logger().Traceln("Initialized StandAlonePubSubManager")
	return nil
}

func (s *StandAlonePubSubManager) Subscribe(channelName string, receiveChannel chan<- ServerMessage) {
	s.mutex.Lock()
	if _, ok := s.Channels[channelName]; !ok {
		s.Channels[channelName] = pubSubChannelConnection{
			ReceiveChannel:  receiveChannel,
			InternalChannel: make(chan localPubSubMessage),
		}
	}
	s.mutex.Unlock()

	for {
		select {
		case msg := <-s.Channels[channelName].InternalChannel:
			receiveChannel <- msg.Message
		}
	}
}

func (s *StandAlonePubSubManager) Publish(channel string, msg ServerMessage) error {
	s.mutex.Lock()
	if _, ok := s.Channels[channel]; !ok {
		log.Logger().Warnf("Channel %s not subscribed to", channel)
		s.mutex.Unlock()
		return nil
	}
	s.mutex.Unlock()
	s.Channels[channel].InternalChannel <- localPubSubMessage{
		Message: msg,
		Channel: channel,
	}
	return nil
}
