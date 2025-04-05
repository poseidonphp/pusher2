package pubsub

import (
	"encoding/json"
	"github.com/go-redis/redis"
	"pusher/internal/constants"
	"pusher/log"
)

type RedisPubSubManager struct {
	PubSubCore
	Client redis.UniversalClient
}

func (r *RedisPubSubManager) SetKeyPrefix(prefix string) {
	r.keyPrefix = prefix
}

func (r *RedisPubSubManager) Publish(channelName constants.ChannelName, message ServerMessage) error {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Logger().Errorf("Could not marshal message: %s", message.Event)
		return err
	}
	r.Client.Publish(string(channelName), string(msgBytes))
	return nil
}

func (r *RedisPubSubManager) Subscribe(channelName constants.ChannelName, receiveChannel chan<- ServerMessage) {
	s := r.Client.Subscribe(string(channelName))
	if s == nil {
		log.Logger().Error("Could not subscribe to Redis channel")
		return
	}
	defer func() {
		log.Logger().Info("Closing Redis subscription")
		_ = s.Close()
	}()
	for {
		msgI, err := s.Receive()
		if err != nil {
			log.Logger().Error(err)
			return
		}
		switch msg := msgI.(type) {
		case *redis.Message:
			var sMsg ServerMessage
			umErr := json.Unmarshal([]byte(msg.Payload), &sMsg)
			if umErr != nil {
				log.Logger().Errorf("Could not unmarshal message: %v", umErr)
				continue
			}
			receiveChannel <- sMsg
		case *redis.Subscription:
			log.Logger().Infof("Subscription message: %s %s %d", msg.Channel, msg.Kind, msg.Count)
		default:
			log.Logger().Errorf("Unknown message type: %v", msg)
		}
	}
}
