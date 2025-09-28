package queues

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"pusher/internal/clients"
	"pusher/internal/webhooks"
	"pusher/log"

	"github.com/redis/go-redis/v9"
)

func NewRedisQueue(ctx context.Context, conn *clients.RedisClient, prefix string, webhookSender *webhooks.WebhookSender) (*RedisQueue, error) {
	if conn == nil || conn.Client == nil {
		return nil, errors.New("redis client is nil")
	}
	r := &RedisQueue{
		RedisClient: conn,
	}
	aq, err := NewAbstractQueue(ctx, r, webhookSender, true, 3*time.Second)
	if err != nil {
		log.Logger().Errorf("Error creating AbstractQueue: %v", err)
		return nil, err
	}
	r.AbstractQueue = aq
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

func (rd *RedisQueue) Shutdown(ctx context.Context) {
	queueName := rd.RedisClient.GetKey("webhook_queue")
	workingQueue := queueName + "_working"

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Logger().Warn("Redis dispatcher shutdown timed out, exiting with jobs still in working queue")
			return
		case <-ticker.C:
			lenCmd := rd.RedisClient.Client.LLen(ctx, workingQueue)
			if err := lenCmd.Err(); err != nil {
				log.Logger().Errorf("Error checking working queue length: %v", err)
				continue
			}
			if lenCmd.Val() == 0 {
				log.Logger().Debug("Redis dispatcher shut down gracefully, working queue is empty")
				return
			}
		}
	}
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
	}
}

func (rd *RedisQueue) processEvent(rawEventString string, queueName string) {
	// Deserialize the JSON data back to webhook event

	var payload webhooks.QueuedJobData
	err := json.Unmarshal([]byte(rawEventString), &payload)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling event data: %s", err)
		// Remove the malformed event from the working queue to prevent blocking
		r := rd.removeEventFromQueue(rawEventString, queueName)
		if r != nil {
			log.Logger().Errorf("Error removing malformed event from working queue: %s", r.Error())
		} else {
			log.Logger().Tracef("Successfully removed malformed event from working queue")
		}
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
	r := rd.removeEventFromQueue(rawEventString, queueName)
	if r != nil {
		log.Logger().Errorf("Error removing event from working queue: %s", r.Error())
	} else {
		log.Logger().Tracef("Successfully removed event from working queue")
	}
}

func (rd *RedisQueue) removeEventFromQueue(rawEventString, queueName string) error {
	r := rd.RedisClient.Client.LRem(context.Background(), queueName+"_working", 1, rawEventString)
	if r.Err() != nil {
		log.Logger().Errorf("Error removing event from working queue: %s", r.Err())
		return r.Err()
	}
	log.Logger().Tracef("Successfully removed event from working queue")
	return nil
}
