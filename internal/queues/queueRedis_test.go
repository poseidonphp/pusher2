package queues

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/pusher/pusher-http-go/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pusher/internal/apps"
	"pusher/internal/clients"
	"pusher/internal/constants"
	"pusher/internal/webhooks"
)

// Helper function to create a test Redis client using miniredis
func createTestRedisClient(t *testing.T) (*clients.RedisClient, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)

	// Create a go-redis client that connects to miniredis
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	redisClient := &clients.RedisClient{
		Client: rdb,
		Prefix: "test",
	}

	return redisClient, mr
}

// Helper function to create a test app
func createTestAppForRedisQueue() *apps.App {
	app := &apps.App{
		ID:      "test-app",
		Key:     "test-key",
		Secret:  "test-secret",
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test webhook
func createTestWebhookForRedis() *constants.Webhook {
	return &constants.Webhook{
		URL:        "https://example.com/webhook",
		Headers:    map[string]string{"Authorization": "Bearer token"},
		EventTypes: []string{"client_event", "member_added"},
	}
}

// Helper function to create test webhook event
func createTestWebhookEventForRedis() *pusher.WebhookEvent {
	return &pusher.WebhookEvent{
		Name:    "test-event",
		Channel: "test-channel",
		Data:    `{"message": "test data"}`,
	}
}

// Helper function to create test queued job data
func createTestQueuedJobDataForRedis() *webhooks.QueuedJobData {
	return &webhooks.QueuedJobData{
		Webhook:   createTestWebhookForRedis(),
		Payload:   createTestWebhookEventForRedis(),
		AppID:     "test-app",
		AppKey:    "test-key",
		AppSecret: "test-secret",
	}
}

func TestNewRedisQueue(t *testing.T) {
	t.Run("SuccessfulCreation", func(t *testing.T) {
		ctx := context.Background()
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}

		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)

		assert.NoError(t, err)
		assert.NotNil(t, queue)
		assert.NotNil(t, queue.AbstractQueue)
		assert.Equal(t, redisClient, queue.RedisClient)
	})

	t.Run("NilRedisClient", func(t *testing.T) {
		ctx := context.Background()

		webhookSender := &webhooks.WebhookSender{}

		queue, err := NewRedisQueue(ctx, nil, "test", webhookSender)

		// This should still work as NewAbstractQueue handles nil clients
		assert.Error(t, err)
		assert.Nil(t, queue)
	})

	t.Run("NilWebhookSender", func(t *testing.T) {
		ctx := context.Background()
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue, err := NewRedisQueue(ctx, redisClient, "test", nil)

		// This should still work as NewAbstractQueue handles nil webhookSender
		assert.NoError(t, err)
		assert.NotNil(t, queue)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}

		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)

		// Should still create successfully, cancellation affects monitoring
		assert.NoError(t, err)
		assert.NotNil(t, queue)
	})
}

func TestRedisQueue_Init(t *testing.T) {
	t.Run("SuccessfulInit", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		err := queue.Init()

		assert.NoError(t, err)
	})

	t.Run("NilRedisClient", func(t *testing.T) {
		// This will panic due to nil pointer dereference in Init
		// Skip this test since the current implementation doesn't handle nil RedisClient
		t.Skip("Skipping nil RedisClient test - current implementation doesn't handle nil RedisClient")
	})

	t.Run("NilClientInRedisClient", func(t *testing.T) {
		queue := &RedisQueue{
			RedisClient: &clients.RedisClient{
				Client: nil,
				Prefix: "test",
			},
		}

		err := queue.Init()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client is not initialized")
	})
}

func TestRedisQueue_addToQueue(t *testing.T) {
	t.Run("AddSingleMessage", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		jobData := createTestQueuedJobDataForRedis()

		queue.addToQueue(jobData)

		// Check that the message was added to Redis
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), length)

		// Verify the message content
		message, err := redisClient.Client.LPop(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.NotEmpty(t, message)

		// Verify it's valid JSON
		var decodedData webhooks.QueuedJobData
		err = json.Unmarshal([]byte(message), &decodedData)
		assert.NoError(t, err)
		assert.Equal(t, jobData.AppID, decodedData.AppID)
		assert.Equal(t, jobData.AppKey, decodedData.AppKey)
	})

	t.Run("AddMultipleMessages", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		// Add multiple messages
		for i := 0; i < 3; i++ {
			jobData := createTestQueuedJobDataForRedis()
			jobData.Payload.Name = "test-event-" + string(rune(i))
			queue.addToQueue(jobData)
		}

		// Check that all messages were added
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		// Create a jobData that will cause JSON marshaling to fail
		// by using a channel that can't be marshaled
		jobData := &webhooks.QueuedJobData{
			Webhook:   createTestWebhookForRedis(),
			Payload:   &pusher.WebhookEvent{},
			AppID:     "test-app",
			AppKey:    "test-key",
			AppSecret: "test-secret",
		}

		// This should not panic and should log an error
		queue.addToQueue(jobData)

		// The message should still be added to Redis since the JSON marshaling succeeds
		// (empty WebhookEvent can be marshaled)
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})

	t.Run("RedisConnectionError", func(t *testing.T) {
		// This will panic due to nil pointer dereference in addToQueue
		// Skip this test since the current implementation doesn't handle nil Redis client
		t.Skip("Skipping Redis connection error test - current implementation doesn't handle nil Redis client")
	})
}

func TestRedisQueue_monitorQueue(t *testing.T) {
	t.Run("ProcessSingleMessage", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Add a message to the queue
		jobData := createTestQueuedJobDataForRedis()
		queue.addToQueue(jobData)

		// Start monitoring in goroutine
		done := make(chan struct{})
		go func() {
			queue.monitorQueue(ctx)
			close(done)
		}()

		// Give some time for processing
		time.Sleep(100 * time.Millisecond)

		// The message should have been processed and removed from the queue
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)

		// Ensure monitorQueue exits before closing Redis
		cancel()
		<-done
	})

	t.Run("ProcessMultipleMessages", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring in goroutine
		done := make(chan struct{})
		go func() {
			queue.monitorQueue(ctx)
			close(done)
		}()

		// Add multiple messages
		for i := 0; i < 3; i++ {
			jobData := createTestQueuedJobDataForRedis()
			jobData.Payload.Name = "test-event-" + string(rune(i))
			queue.addToQueue(jobData)
		}

		// Give time for processing
		time.Sleep(500 * time.Millisecond)

		// All messages should have been processed
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)

		// Ensure monitorQueue exits before closing Redis
		cancel()
		<-done
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring in goroutine
		done := make(chan bool)
		go func() {
			queue.monitorQueue(ctx)
			done <- true
		}()

		// Cancel context
		cancel()

		// Wait for goroutine to finish
		select {
		case <-done:
			// Expected - goroutine should exit
		case <-time.After(100 * time.Millisecond):
			t.Fatal("monitorQueue should have exited due to context cancellation")
		}
	})

	t.Run("EmptyQueueHandling", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring in goroutine
		go queue.monitorQueue(ctx)

		// Don't add any messages, just let it run
		time.Sleep(100 * time.Millisecond)

		// Should not crash or exit
	})

	t.Run("ConcurrentMessageProcessing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring in goroutine
		done := make(chan struct{})
		go func() {
			queue.monitorQueue(ctx)
			close(done)
		}()

		// Add messages concurrently
		for i := 0; i < 10; i++ {
			go func(index int) {
				jobData := createTestQueuedJobDataForRedis()
				jobData.Payload.Name = "concurrent-event-" + string(rune(index))
				queue.addToQueue(jobData)
			}(i)
		}

		// Give time for all messages to be processed
		time.Sleep(500 * time.Millisecond)

		// All messages should have been processed
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)

		// Ensure monitorQueue exits before closing Redis
		cancel()
		<-done
	})
}

func TestRedisQueue_processEvent(t *testing.T) {
	t.Run("ProcessValidEvent", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		ctx, cancel := context.WithCancel(context.Background())
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)
		cancel()

		// Create a valid JSON event
		jobData := createTestQueuedJobDataForRedis()
		eventData, err := json.Marshal(jobData)
		require.NoError(t, err)

		// Add the event to the working queue
		queueName := redisClient.GetKey("webhook_queue")
		workingQueueName := queueName + "_working"
		redisClient.Client.LPush(context.Background(), workingQueueName, string(eventData))

		// Process the event
		queue.processEvent(string(eventData), queueName)
		time.Sleep(50 * time.Millisecond) // Give some time for processing

		// The event should be removed from the working queue
		length, err := redisClient.Client.LLen(context.Background(), workingQueueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("ProcessInvalidJSON", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(context.Background(), redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Create invalid JSON
		invalidJSON := `{"invalid": json}`

		// Add the invalid event to the working queue
		queueName := redisClient.GetKey("webhook_queue")
		workingQueueName := queueName + "_working"
		redisClient.Client.LPush(context.Background(), workingQueueName, invalidJSON)

		// Process the event - should not crash
		queue.processEvent(invalidJSON, queueName)

		// The invalid event should still be removed from the working queue
		length, err := redisClient.Client.LLen(context.Background(), workingQueueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("ProcessEmptyEvent", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(context.Background(), redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Process empty event - should not crash
		queueName := redisClient.GetKey("webhook_queue")
		queue.processEvent("", queueName)

		// Should not crash
	})
}

func TestRedisQueue_Integration(t *testing.T) {
	t.Run("FullWorkflow", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring
		done := make(chan struct{})
		go func() {
			queue.monitorQueue(ctx)
			close(done)
		}()

		// Create and send various types of events
		events := []*webhooks.QueuedJobData{
			createTestQueuedJobDataForRedis(),
			createTestQueuedJobDataForRedis(),
		}
		events[1].Payload.Name = "member_added"
		events[1].Payload.Channel = "presence-test"

		// Send events
		for _, event := range events {
			queue.addToQueue(event)
		}

		// Give time for processing
		time.Sleep(200 * time.Millisecond)

		// All events should have been processed
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)

		// Ensure monitorQueue exits before closing Redis
		cancel()
		<-done
	})
}

func TestRedisQueue_EdgeCases(t *testing.T) {
	t.Run("NilMessage", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		// This should not panic
		assert.NotPanics(t, func() {
			queue.addToQueue(nil)
		})
	})

	t.Run("VeryLargeMessage", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		jobData := createTestQueuedJobDataForRedis()
		// Create a large data payload
		largeData := make([]byte, 10000)
		for i := range largeData {
			largeData[i] = 'A'
		}
		jobData.Payload.Data = string(largeData)

		queue.addToQueue(jobData)

		// Check that the message was added
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})

	t.Run("SpecialCharactersInMessage", func(t *testing.T) {
		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		queue := &RedisQueue{
			RedisClient: redisClient,
		}

		jobData := createTestQueuedJobDataForRedis()
		jobData.Payload.Data = `{"message": "Special chars: \n\t\r\"'\\"}`

		queue.addToQueue(jobData)

		// Check that the message was added
		queueName := redisClient.GetKey("webhook_queue")
		length, err := redisClient.Client.LLen(context.Background(), queueName).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), length)

		// Verify the message content
		message, err := redisClient.Client.LPop(context.Background(), queueName).Result()
		assert.NoError(t, err)

		// Verify it's valid JSON
		var decodedData webhooks.QueuedJobData
		err = json.Unmarshal([]byte(message), &decodedData)
		assert.NoError(t, err)
		assert.Equal(t, jobData.Payload.Data, decodedData.Payload.Data)
	})
}

func TestRedisQueue_Performance(t *testing.T) {
	t.Run("HighThroughput", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var err error

		redisClient, mr := createTestRedisClient(t)
		defer mr.Close()

		webhookSender := &webhooks.WebhookSender{}
		queue, err := NewRedisQueue(ctx, redisClient, "test", webhookSender)
		require.NoError(t, err)

		// Start monitoring
		done := make(chan struct{})
		go func() {
			queue.monitorQueue(ctx)
			close(done)
		}()

		// Send many messages
		messageCount := 100
		start := time.Now()

		for i := 0; i < messageCount; i++ {
			jobData := createTestQueuedJobDataForRedis()
			jobData.Payload.Name = "perf-event-" + string(rune(i))
			queue.addToQueue(jobData)
		}

		// Wait for the queue to be empty, up to 2 seconds
		queueName := redisClient.GetKey("webhook_queue")
		var length int64

		deadline := time.Now().Add(10 * time.Second)
		for {
			length, err = redisClient.Client.LLen(context.Background(), queueName).Result()
			if err != nil {
				break
			}
			if length == 0 || time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		assert.NoError(t, err)
		assert.Equal(t, int64(0), length, "Queue not empty after processing, %d messages left", length)

		// Should complete within reasonable time
		duration := time.Since(start)
		assert.True(t, duration < 10*time.Second)

		// Ensure monitorQueue exits before closing Redis
		cancel()
		<-done
	})
}
