package queues

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"pusher/internal/clients"
	"pusher/internal/webhooks"
	"pusher/log"
)

func NewRedisQueue(ctx context.Context, conn *clients.RedisClient, prefix string, webhookSender *webhooks.WebhookSender) (*RedisQueue, error) {
	r := &RedisQueue{
		RedisClient: conn,
	}
	aq, err := NewAbstractQueue(ctx, r, webhookSender, true, 3*time.Second)
	if err != nil {
		log.Logger().Errorf("Error creating AbstractQueue: %v", err)
		return nil, err
	}
	r.AbstractQueue = aq
	// err = r.Init() // this is called by the NewAbstractQueue()
	// if err != nil {
	// 	log.Logger().Errorf("Error initializing Redis dispatcher: %v", err)
	// 	return nil, err
	// }
	return r, nil
}

type RedisQueue struct {
	// QueueCore
	*AbstractQueue
	RedisClient *clients.RedisClient
	// WebhookManager webhooks.WebhookInterface
}

func (rd *RedisQueue) Init() error {
	if rd.RedisClient.Client == nil {
		return errors.New("redis client is not initialized")
	}
	return nil
}

func (rd *RedisQueue) addToQueue(jobData *webhooks.QueuedJobData) {
	// Serialize the server event to JSON
	eventData, err := json.Marshal(jobData)
	if err != nil {
		log.Logger().Errorf("Error marshalling server event: %s", err)
		return
	}

	// Push the event data to the Redis queue
	queueName := rd.RedisClient.GetKey("webhook_queue")

	err = rd.RedisClient.Client.LPush(context.Background(), queueName, eventData).Err()
	if err != nil {
		log.Logger().Errorf("Error pushing event to Redis queue: %s", err)
	}
}

func (rd *RedisQueue) monitorQueue(ctx context.Context) {
	queueName := rd.RedisClient.GetKey("webhook_queue")
	log.Logger().Infoln("Starting to listen for events on queue:", queueName)

	// Use a shorter timeout to allow checking for context cancellation
	const redisPollTimeout = 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Logger().Infoln("Stopping Redis dispatcher due to context cancellation")
			return
		default:
			eventData, err := rd.RedisClient.Client.BRPopLPush(context.Background(), queueName, queueName+"_working", redisPollTimeout).Result()

			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				log.Logger().Errorf("Error getting event from Redis queue: %s", err)
				// Sleep to prevent tight loop in case of persistent errors
				time.Sleep(1 * time.Second)
				continue
			}
			rd.processEvent(eventData, queueName)
		}
		// // Use BRPOPLPUSH to block until a message is available and atomically pop it into a new temp queue
		// // The timeout of 0 means block indefinitely
		// eventData, err := rd.Client.BRPopLPush(queueName, queueName+"_working", 0).Result()
		// if err != nil {
		//	log.Logger().Errorf("Error receiving event from Redis queue: %s", err)
		//	// Sleep to prevent tight loop in case of persistent errors
		//	time.Sleep(1 * time.Second)
		//	continue
		// }
		//
		// rd.processEvent(eventData, queueName)
	}
}

func (rd *RedisQueue) processEvent(rawEventString string, queueName string) {
	// Deserialize the JSON data back to webhook event
	// var webhookEvent pusher.WebhookEvent
	var payload webhooks.QueuedJobData
	err := json.Unmarshal([]byte(rawEventString), &payload)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling event data: %s", err)
		return
	}

	// Process the event
	log.Logger().Debugf("Processing webhook event: %s (%s)", payload.Payload.Name, payload.Payload.Channel)

	// wh := &pusher.Webhook{
	// 	TimeMs: int(time.Now().UnixMilli()),
	// 	Events: []pusher.WebhookEvent{payload.Payload},
	// }
	// TODO: Handle error codes such as too many requests, not-found. Retry if needed but keep count of attempts

	rd.sendWebhook(&payload)

	log.Logger().Tracef(".. removing %s from working queue", rawEventString)
	r := rd.RedisClient.Client.LRem(context.Background(), queueName+"_working", 1, rawEventString)
	if r.Err() != nil {
		log.Logger().Errorf("Error removing event from working queue: %s", r.Err())
	} else {
		log.Logger().Tracef("Successfully removed event from working queue")
	}
}
