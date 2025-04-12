package pubsub

import (
	"context"
	"sync"
)

type localPubSubMessage struct {
	Message ServerMessage
	Channel string
}

//type pubSubChannelConnection struct {
//	ReceiveChannel  chan<- ServerMessage
//	InternalChannel chan localPubSubMessage
//}

//type StandAlonePubSubManager struct {
//	PubSubCore
//	ch       chan localPubSubMessage
//	Channels map[string]pubSubChannelConnection
//	mutex    sync.Mutex
//}

//func (s *StandAlonePubSubManager) Init() error {
//	s.ch = make(chan localPubSubMessage, 100)
//	s.Channels = make(map[string]pubSubChannelConnection)
//	log.Logger().Traceln("Initialized StandAlonePubSubManager")
//	return nil
//}

//func (s *StandAlonePubSubManager) Subscribe(channelName string, receiveChannel chan<- ServerMessage) {
//	s.mutex.Lock()
//	if _, ok := s.Channels[channelName]; !ok {
//		s.Channels[channelName] = pubSubChannelConnection{
//			ReceiveChannel:  receiveChannel,
//			InternalChannel: make(chan localPubSubMessage),
//		}
//	}
//	s.mutex.Unlock()
//
//	for {
//		select {
//		case msg := <-s.Channels[channelName].InternalChannel:
//			receiveChannel <- msg.Message
//		}
//	}
//}

//func (s *StandAlonePubSubManager) Publish(ctx context.Context, channelName string, message ServerMessage) error {
//	if ctx == nil {
//		log.Logger().Error("Context is nil")
//		return nil
//	}
//	s.mutex.Lock()
//	if _, ok := s.Channels[channelName]; !ok {
//		log.Logger().Warnf("Channel %s not subscribed to", channelName)
//		s.mutex.Unlock()
//		return nil
//	}
//	s.mutex.Unlock()
//	newLocalMessage := &localPubSubMessage{
//		Message: message,
//		Channel: channelName,
//	}
//	select {
//	case s.Channels[channelName].InternalChannel <- *newLocalMessage:
//		return nil
//	case <-ctx.Done():
//		return ctx.Err()
//	}
//}

type pubSubChannelConnection struct {
	ReceiveChannel chan<- ServerMessage
	//SocketID       constants.SocketID // Optional socket ID for filtering
}

type StandAlonePubSubManager struct {
	PubSubCore
	// Map of channel names to a list of subscribers
	subscriptions map[string][]pubSubChannelConnection
	mutex         sync.RWMutex
}

func (s *StandAlonePubSubManager) SubscribeWithNotify(ctx context.Context, channelName string, receiveChannel chan<- ServerMessage, ready chan<- struct{}) {
	channelKey := s.getKeyName(channelName)

	s.mutex.Lock()
	subscription := pubSubChannelConnection{
		ReceiveChannel: receiveChannel,
	}
	s.subscriptions[channelKey] = append(s.subscriptions[channelKey], subscription)
	s.mutex.Unlock()

	// Signal that subscription is ready
	close(ready)

	// Block until context is done
	<-ctx.Done()
	s.removeSubscriber(channelKey, subscription)
}

func (s *StandAlonePubSubManager) Init() error {
	s.subscriptions = make(map[string][]pubSubChannelConnection)
	return nil
}

func (s *StandAlonePubSubManager) Subscribe(ctx context.Context, channelName string, receiveChannel chan<- ServerMessage) {
	channelKey := s.getKeyName(channelName)

	s.mutex.Lock()
	subscription := pubSubChannelConnection{
		ReceiveChannel: receiveChannel,
	}
	s.subscriptions[channelKey] = append(s.subscriptions[channelKey], subscription)
	s.mutex.Unlock()

	// Block until channel is closed or context is done
	//<-make(chan struct{})
	<-ctx.Done()
	s.removeSubscriber(channelKey, subscription)
}

func (s *StandAlonePubSubManager) Publish(ctx context.Context, channelName string, message ServerMessage) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	channelKey := s.getKeyName(channelName)

	s.mutex.RLock()
	if _, ok := s.subscriptions[channelKey]; !ok {
		s.mutex.RUnlock()
		return nil // No subscribers for this channel
	}
	subscribers := make([]pubSubChannelConnection, len(s.subscriptions[channelKey]))
	copy(subscribers, s.subscriptions[channelKey])
	s.mutex.RUnlock()

	var failedSubscribers []pubSubChannelConnection

	for _, sub := range subscribers {
		// Safely send to the channel using a closure with recover
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel is closed
					failedSubscribers = append(failedSubscribers, sub)
				}
			}()

			// Non-blocking send with timeout
			select {
			case sub.ReceiveChannel <- message:
				// Message sent successfully
			case <-ctx.Done():
				panic("context done") // Will be recovered
			default:
				// Channel is full
				failedSubscribers = append(failedSubscribers, sub)
			}
		}()
	}

	// clean up failed subscribers
	for _, failedSub := range failedSubscribers {
		s.removeSubscriber(channelKey, failedSub)
	}

	return nil
}

func (s *StandAlonePubSubManager) removeSubscriber(channelKey string, subToRemove pubSubChannelConnection) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	subscribers := s.subscriptions[channelKey]
	for i, sub := range subscribers {
		if sub.ReceiveChannel == subToRemove.ReceiveChannel {
			// Remove subscriber by replacing it with the last one and truncating
			subscribers[i] = subscribers[len(subscribers)-1]
			s.subscriptions[channelKey] = subscribers[:len(subscribers)-1]
			break
		}
	}
}
