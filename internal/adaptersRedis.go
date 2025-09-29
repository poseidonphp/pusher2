package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"pusher/internal/metrics"
	"pusher/log"

	"github.com/redis/go-redis/v9"
)

func NewRedisAdapter(ctx context.Context, conn redis.UniversalClient, prefix string, channel string, metricsManager metrics.MetricsInterface) (*RedisAdapter, error) {
	r := &RedisAdapter{
		ctx:       ctx,
		subClient: conn,
		pubClient: conn,
		Prefix:    prefix,
		Channel:   channel,
	}
	// Create the horizontal adapter, injecting this instance that satisfies the HorizontalInterface
	ha, err := NewHorizontalAdapter(r, metricsManager)
	if err != nil {
		log.Logger().Errorf("Error creating HorizontalAdapter: %v", err)
		return nil, err
	}
	r.HorizontalAdapter = ha

	// Initialize the Redis adapter
	err = r.Init()
	if err != nil {
		log.Logger().Errorf("Error initializing Redis adapter: %v", err)
		return nil, err
	}
	return r, nil
}

type RedisAdapter struct {
	*HorizontalAdapter
	ctx       context.Context
	Channel   string
	subClient redis.UniversalClient
	pubClient redis.UniversalClient
	Prefix    string
}

func (ra *RedisAdapter) Init() error {
	if ra.subClient == nil || ra.pubClient == nil {
		return errors.New("redis client not initialized")
	}
	if ra.Prefix != "" {
		ra.Channel = fmt.Sprintf("%s#%s", ra.Prefix, ra.Channel)
	}

	ra.requestChannel = ra.Channel + "#comms#req"
	ra.responseChannel = ra.Channel + "#comms#res"

	ra.psubscribe()
	return nil
}

func (ra *RedisAdapter) GetChannelName() string {
	return ra.Channel
}

func (ra *RedisAdapter) broadcastToChannel(channel string, data string) {
	_ = ra.pubClient.Publish(ra.ctx, channel, data)
}

// processMessage handles incoming messages from the request/response channel
func (ra *RedisAdapter) processMessage(redisChannel string, message string) {
	if strings.HasPrefix(redisChannel, ra.responseChannel) {
		ra.onResponse(redisChannel, message)
	} else if strings.HasPrefix(redisChannel, ra.requestChannel) {
		log.Logger().Tracef(" ... Step 1 calling onRequest()")
		dataToSend, err := ra.onRequest(redisChannel, message)
		log.Logger().Tracef(" ... Step 2 got response from onRequest(): %s", dataToSend)
		if err != nil {
			log.Logger().Errorf("Error processing request: %v", err)
			return
		}
		if dataToSend != nil {
			log.Logger().Warnf("Broadcasting to channel %s: %s", ra.responseChannel, dataToSend)
			ra.broadcastToChannel(ra.responseChannel, string(dataToSend))
		}
	}
}

// onMessage handles incoming messages from the pattern match subscription;
// Used for other nodes sending messages to local sockets
func (ra *RedisAdapter) onMessage(pattern string, redisChannel string, message string) {
	// This Channel is just for the en-masse broadcasting, not for processing
	// the request-response cycle to gather info across multiple nodes.
	if !strings.HasPrefix(redisChannel, ra.Channel) {
		return
	}

	var decodedMessage *horizontalPubsubBroadcastedMessage
	err := json.Unmarshal([]byte(message), &decodedMessage)
	if err != nil {
		log.Logger().Errorf("Could not unmarshal message: %v", err)
		return
	}

	if ra.uuid == decodedMessage.Uuid || decodedMessage.AppID == "" || decodedMessage.Channel == "" || decodedMessage.Data == "" {
		return
	}

	_data := decodedMessage.Data.(string)

	ra.sendLocally(decodedMessage.AppID, decodedMessage.Channel, _data, decodedMessage.ExceptingIds)
}

// getNumSub gets the number of redis subscribers
func (ra *RedisAdapter) getNumSub() (int64, error) {
	res := ra.pubClient.PubSubNumSub(ra.ctx, ra.requestChannel)
	if res.Err() != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", res.Err())
		return 0, res.Err()
	}
	numSub := res.Val()

	if val, exists := numSub[ra.requestChannel]; exists {
		log.Logger().Tracef("Number of subscribers: %v", val)
		return val, nil
	}
	log.Logger().Debugf("error with number of subscribers: %v", numSub)
	return 0, nil
}

func (ra *RedisAdapter) disconnect() {
	ra.subClient.Quit(ra.ctx)
	ra.pubClient.Quit(ra.ctx)
}

func (ra *RedisAdapter) psubscribe() {
	sub := ra.subClient.PSubscribe(ra.ctx, ra.Channel+"*")
	directSubs := ra.subClient.Subscribe(ra.ctx, ra.responseChannel, ra.requestChannel)

	go func() {
		log.Logger().Trace("psubscribe to channel: ", ra.Channel)
		defer func() {
			log.Logger().Trace("closing psubscribe")
		}()
		for {
			msgI, err := sub.Receive(ra.ctx)
			if err != nil {
				log.Logger().Errorf("psubscribe err: %v", err)
				return
			}

			switch msg := msgI.(type) {
			case *redis.Message:
				// Process the pattern-matched message
				log.Logger().Tracef("received message on psub channel %s: %s", msg.Pattern, msg.Payload)
				ra.onMessage(msg.Pattern, msg.Channel, msg.Payload)
			}
		}
	}()

	// todo add line 68 from redis-adapter.ts?

	go func() {
		log.Logger().Trace("subscribe to request/response channel: ", ra.Channel)
		defer func() {
			log.Logger().Trace("closing subscribe")
		}()
		for {
			msgI, err := directSubs.Receive(ra.ctx)
			if err != nil {
				log.Logger().Errorf("subscribe err: %v", err)
				return
			}

			switch msg := msgI.(type) {
			case *redis.Message:
				// Process the direct Channel message
				log.Logger().Tracef("Received message on req/res channel %s: %s", msg.Channel, msg.Payload)
				ra.processMessage(msg.Channel, msg.Payload)
			case *redis.Error:
				log.Logger().Errorf("Error in req/res received message %v", msg)
			}
		}
	}()

}
