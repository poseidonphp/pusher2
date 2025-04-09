package pubsub

import (
	"context"
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
	s.ch = make(chan localPubSubMessage, 100)
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

func (s *StandAlonePubSubManager) Publish(ctx context.Context, channelName string, message ServerMessage) error {
	if ctx == nil {
		log.Logger().Error("Context is nil")
		return nil
	}
	s.mutex.Lock()
	if _, ok := s.Channels[channelName]; !ok {
		log.Logger().Warnf("Channel %s not subscribed to", channelName)
		s.mutex.Unlock()
		return nil
	}
	s.mutex.Unlock()
	lpm := &localPubSubMessage{
		Message: message,
		Channel: channelName,
	}
	select {
	case s.Channels[channelName].InternalChannel <- *lpm:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	//s.Channels[channelName].InternalChannel <- *lpm
	//return nil
}
