package pubsub

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"pusher/log"
)

type RedisPubSubManager struct {
	PubSubCore
	Client    redis.UniversalClient
	KeyPrefix string
}

func (r *RedisPubSubManager) SubscribeWithNotify(ctx context.Context, channelName string, receiveChannel chan<- ServerMessage, ready chan<- struct{}) {
	s := r.Client.Subscribe(context.Background(), channelName)
	if s == nil {
		log.Logger().Error("Could not subscribe to Redis channel")
		return
	}

	defer func() {
		log.Logger().Info("Closing Redis subscription")
		_ = s.Close()
	}()

	close(ready)

	for {
		msgI, err := s.Receive(ctx)
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
			log.Logger().Debugf("Subscription message: %s %s %d", msg.Channel, msg.Kind, msg.Count)

		default:
			log.Logger().Errorf("Unknown message type: %v (%v)", msg, msgI)
		}
	}
}

func (r *RedisPubSubManager) Init() error {
	if r.Client == nil {
		return redis.Nil
	}

	r.keyPrefix = r.KeyPrefix

	if err := r.Client.Ping(context.Background()).Err(); err != nil {
		log.Logger().Errorf("Redis ping failed: %s", err)
		return err
	}

	log.Logger().Debugln("Redis PubSub Manager initialized")
	return nil
}

func (r *RedisPubSubManager) Publish(_ context.Context, channelName string, message ServerMessage) error {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Logger().Errorf("Could not marshal message: %s", message.Event)
		return err
	}

	r.Client.Publish(context.Background(), channelName, string(msgBytes))

	return nil
}

func (r *RedisPubSubManager) Subscribe(ctx context.Context, channelName string, receiveChannel chan<- ServerMessage) {
	s := r.Client.Subscribe(context.Background(), channelName)
	if s == nil {
		log.Logger().Error("Could not subscribe to Redis channel")
		return
	}

	defer func() {
		log.Logger().Info("Closing Redis subscription")
		_ = s.Close()
	}()

	for {
		msgI, err := s.Receive(ctx)
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
			log.Logger().Debugf("Subscription message: %s %s %d", msg.Channel, msg.Kind, msg.Count)

		default:
			log.Logger().Errorf("Unknown message type: %v (%v)", msg, msgI)
		}
	}
}
