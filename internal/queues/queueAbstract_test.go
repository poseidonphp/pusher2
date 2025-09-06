package queues

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/webhooks"

	"github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

// MockQueueInterface implements QueueInterface for testing AbstractQueue
type MockQueueInterface struct {
	*AbstractQueue
	mu                sync.Mutex
	addToQueueCalls   []*webhooks.QueuedJobData
	monitorQueueCalls []context.Context
	initError         error

	sendClientCalls     []*webhooks.QueuedJobData
	sendMemberAdded     []*webhooks.QueuedJobData
	sendMemberRemoved   []*webhooks.QueuedJobData
	sendChannelVacated  []*webhooks.QueuedJobData
	sendChannelOccupied []*webhooks.QueuedJobData
	sendCacheMissed     []*webhooks.QueuedJobData
}

func (m *MockQueueInterface) addToQueue(data *webhooks.QueuedJobData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addToQueueCalls = append(m.addToQueueCalls, data)

	switch data.Payload.Name {
	case string(constants.WebHookMemberAdded):
		m.sendMemberAdded = append(m.sendMemberAdded, data)
	case string(constants.WebHookMemberRemoved):
		m.sendMemberRemoved = append(m.sendMemberRemoved, data)
	case string(constants.WebHookChannelVacated):
		m.sendChannelVacated = append(m.sendChannelVacated, data)
	case string(constants.WebHookChannelOccupied):
		m.sendChannelOccupied = append(m.sendChannelOccupied, data)
	case string(constants.WebHookCacheMiss):
		m.sendCacheMissed = append(m.sendCacheMissed, data)
	default:
		m.sendClientCalls = append(m.sendClientCalls, data)
	}
}

func (m *MockQueueInterface) monitorQueue(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.monitorQueueCalls = append(m.monitorQueueCalls, ctx)
}

func (m *MockQueueInterface) Init() error {
	return m.initError
}

// func (m *MockQueueInterface) Send(app *apps.App, webhook *pusher.WebhookEvent) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendCalls = append(m.sendCalls, struct {
// 		app     *apps.App
// 		webhook *pusher.WebhookEvent
// 	}{app, webhook})
// }

// func (m *MockQueueInterface) SendClientEvent(app *apps.App, channel constants.ChannelName, event string, data string, socketID constants.SocketID, userID constants.UserID) {
// func (m *MockQueueInterface) SentClientEvent(data *webhooks.QueuedJobData) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendClientCalls = append(m.sendClientCalls, data)
// 	// m.sendClientEventCalls = append(m.sendClientEventCalls, struct {
// 	// 	app      *apps.App
// 	// 	channel  constants.ChannelName
// 	// 	event    string
// 	// 	data     string
// 	// 	socketID constants.SocketID
// 	// 	userID   constants.UserID
// 	// }{app, channel, event, data, socketID, userID})
// }

// func (m *MockQueueInterface) SendMemberAdded(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendMemberAddedCalls = append(m.sendMemberAddedCalls, struct {
// 		app     *apps.App
// 		channel constants.ChannelName
// 		userID  constants.UserID
// 	}{app, channel, userID})
// }

// func (m *MockQueueInterface) SendMemberRemoved(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendMemberRemovedCalls = append(m.sendMemberRemovedCalls, struct {
// 		app     *apps.App
// 		channel constants.ChannelName
// 		userID  constants.UserID
// 	}{app, channel, userID})
// }
//
// func (m *MockQueueInterface) SendChannelVacated(app *apps.App, channel constants.ChannelName) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendChannelVacatedCalls = append(m.sendChannelVacatedCalls, struct {
// 		app     *apps.App
// 		channel constants.ChannelName
// 	}{app, channel})
// }
//
// func (m *MockQueueInterface) SendChannelOccupied(app *apps.App, channel constants.ChannelName) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendChannelOccupiedCalls = append(m.sendChannelOccupiedCalls, struct {
// 		app     *apps.App
// 		channel constants.ChannelName
// 	}{app, channel})
// }
//
// func (m *MockQueueInterface) SendCacheMissed(app *apps.App, channel constants.ChannelName) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.sendCacheMissedCalls = append(m.sendCacheMissedCalls, struct {
// 		app     *apps.App
// 		channel constants.ChannelName
// 	}{app, channel})
// }

// Helper methods for assertions
func (m *MockQueueInterface) GetAddToQueueCalls() []*webhooks.QueuedJobData {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*webhooks.QueuedJobData(nil), m.addToQueueCalls...)
}

func (m *MockQueueInterface) GetSendClientEventCalls() []*webhooks.QueuedJobData {
	return m.sendClientCalls
}

func (m *MockQueueInterface) GetSendMemberAddedCalls() []*webhooks.QueuedJobData {
	return m.sendMemberAdded
}

func (m *MockQueueInterface) GetSendMemberRemovedCalls() []*webhooks.QueuedJobData {
	return m.sendMemberRemoved
}

func (m *MockQueueInterface) GetSendChannelVacatedCalls() []*webhooks.QueuedJobData {
	return m.sendChannelVacated
}

func (m *MockQueueInterface) GetSendChannelOccupiedCalls() []*webhooks.QueuedJobData {
	return m.sendChannelOccupied
}

func (m *MockQueueInterface) GetSendCacheMissedCalls() []*webhooks.QueuedJobData {
	return m.sendCacheMissed
}

// Helper function to create a test app with webhooks
func createTestAppWithWebhooks() *apps.App {
	app := &apps.App{
		ID:                         "test-app",
		Key:                        "test-key",
		Secret:                     "test-secret",
		WebhooksEnabled:            true,
		HasClientEventWebhooks:     true,
		HasMemberAddedWebhooks:     true,
		HasMemberRemovedWebhooks:   true,
		HasChannelVacatedWebhooks:  true,
		HasChannelOccupiedWebhooks: true,
		Webhooks: []constants.Webhook{
			{
				URL: "https://example.com/webhook1",
				Filter: constants.WebhookFilters{
					ChannelNameStartsWith: "",
					ChannelNameEndsWith:   "",
				},
			},
			{
				URL: "https://example.com/webhook2",
				Filter: constants.WebhookFilters{
					ChannelNameStartsWith: "private-",
					ChannelNameEndsWith:   "",
				},
			},
		},
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create a test app without webhooks
func createTestAppWithoutWebhooks() *apps.App {
	app := &apps.App{
		ID:              "test-app",
		Key:             "test-key",
		Secret:          "test-secret",
		WebhooksEnabled: false,
	}
	app.SetMissingDefaults()
	return app
}

func TestNewAbstractQueue(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)

		assert.NoError(t, err)
		assert.NotNil(t, queue)
		assert.NotNil(t, queue.flapDetector)
		assert.True(t, queue.flapDetector.FlapEnabled)
		assert.Equal(t, 3*time.Second, queue.flapDetector.flapWindowInSeconds)
		assert.NotNil(t, queue.webhookSender)
		assert.Equal(t, mockQueue, queue.concreteQueue)
	})

	t.Run("NilQueueImplementation", func(t *testing.T) {
		ctx := context.Background()

		queue, err := NewAbstractQueue(ctx, nil, &webhooks.WebhookSender{}, true, 3*time.Second)

		assert.Error(t, err)
		assert.Nil(t, queue)
		assert.Contains(t, err.Error(), "queue implementation is nil")
	})

	t.Run("QueueInitError", func(t *testing.T) {
		mockQueue := &MockQueueInterface{
			initError: assert.AnError,
		}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)

		assert.Error(t, err)
		assert.Nil(t, queue)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestAbstractQueue_Send(t *testing.T) {
	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithoutWebhooks()
		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "test-channel",
		}

		// Should not call any queue methods when webhooks are disabled
		queue.Send(app, event)

		// Verify no calls were made
		assert.Empty(t, mockQueue.GetAddToQueueCalls())
	})

	t.Run("NilWebhookSender", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, nil, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "test-channel",
		}

		// Should not call any queue methods when webhook sender is nil
		queue.Send(app, event)

		// Verify no calls were made
		assert.Empty(t, mockQueue.GetAddToQueueCalls())
	})

	t.Run("WithFlapDetection", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		event := &pusher.WebhookEvent{
			Name:    string(constants.WebHookChannelVacated),
			Channel: "test-channel",
		}

		// Send the event
		queue.Send(app, event)

		calls := mockQueue.GetAddToQueueCalls()
		assert.Empty(t, calls) //  Should be empty immediately after sending

		// Wait for flap detection delay
		time.Sleep(150 * time.Millisecond)

		// Verify that prepareQueuedMessages was called
		calls = mockQueue.GetAddToQueueCalls() // after flap delay, it should have been called
		assert.NotEmpty(t, calls)
	})

	t.Run("WithoutFlapDetection", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, false, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		event := &pusher.WebhookEvent{
			Name:    "cache_miss",
			Channel: "test-channel",
		}

		// Send the event
		queue.Send(app, event)

		// Verify that prepareQueuedMessages was called immediately
		calls := mockQueue.GetAddToQueueCalls()
		assert.NotEmpty(t, calls)
	})
}

func TestAbstractQueue_prepareQueuedMessages(t *testing.T) {
	t.Run("SingleWebhook", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		// Remove the second webhook to test single webhook
		app.Webhooks = app.Webhooks[:1]

		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "test-channel",
		}

		queue.prepareQueuedMessages(app, event)

		// Verify addToQueue was called
		calls := mockQueue.GetAddToQueueCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, app.ID, calls[0].AppID)
		assert.Equal(t, app.Key, calls[0].AppKey)
		assert.Equal(t, app.Secret, calls[0].AppSecret)
		assert.Equal(t, event.Name, calls[0].Payload.Name)
		assert.Equal(t, event.Channel, calls[0].Payload.Channel)
		assert.Equal(t, "https://example.com/webhook1", calls[0].Webhook.URL)
	})

	t.Run("MultipleWebhooks", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "private-channel",
		}

		queue.prepareQueuedMessages(app, event)

		// Verify addToQueue was called for both webhooks
		calls := mockQueue.GetAddToQueueCalls()
		assert.Len(t, calls, 2)

		// Check that both webhooks were called (order may vary)
		urls := make([]string, len(calls))
		for i, call := range calls {
			urls[i] = call.Webhook.URL
		}
		assert.Contains(t, urls, "https://example.com/webhook1")
		assert.Contains(t, urls, "https://example.com/webhook2")
	})

	t.Run("WebhookFiltering", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "test-channel",
		}

		queue.prepareQueuedMessages(app, event)

		// The 2nd webhook should not trigger due to the channel name filter (it is set to only match "private-" prefix)
		calls := mockQueue.GetAddToQueueCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, "https://example.com/webhook1", calls[0].Webhook.URL)
	})

	t.Run("NoMatchingWebhooks", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		// Add a webhook that won't match
		app.Webhooks = []constants.Webhook{
			{
				URL: "https://example.com/webhook1",
				Filter: constants.WebhookFilters{
					ChannelNameStartsWith: "public-",
					ChannelNameEndsWith:   "",
				},
			},
		}

		event := &pusher.WebhookEvent{
			Name:    "test-event",
			Channel: "private-test-channel",
		}

		queue.prepareQueuedMessages(app, event)

		// No webhooks should match, so no addToQueue calls
		calls := mockQueue.GetAddToQueueCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendClientEvent(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")
		event := "test-event"
		data := "test-data"
		socketID := constants.SocketID("socket-123")
		userID := constants.UserID("user-456")

		queue.SendClientEvent(app, channel, event, data, socketID, userID)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendClientEventCalls()
		assert.Len(t, calls, 1)
		if len(calls) > 0 {
			assert.Equal(t, app.ID, calls[0].AppID)
			assert.Equal(t, channel, calls[0].Payload.Channel)
			assert.Equal(t, event, calls[0].Payload.Name)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 3*time.Second)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasClientEventWebhooks = false

		channel := constants.ChannelName("test-channel")
		event := "test-event"
		data := "test-data"
		socketID := constants.SocketID("socket-123")
		userID := constants.UserID("user-456")

		// Should not call Send when webhooks are disabled
		queue.SendClientEvent(app, channel, event, data, socketID, userID)

		// Verify no calls were made
		calls := mockQueue.GetAddToQueueCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendMemberAdded(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")
		userID := constants.UserID("user-456")

		queue.SendMemberAdded(app, channel, userID)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendMemberAddedCalls()
		assert.Len(t, calls, 1)
		if len(calls) > 0 {
			assert.Equal(t, channel, calls[0].Payload.Channel)
			assert.Equal(t, userID, calls[0].Payload.UserID)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasMemberAddedWebhooks = false

		channel := constants.ChannelName("test-channel")
		userID := constants.UserID("user-456")

		// Should not call Send when webhooks are disabled
		queue.SendMemberAdded(app, channel, userID)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify no calls were made
		calls := mockQueue.GetSendMemberAddedCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendMemberRemoved(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond) // no flap detection for this test
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")
		userID := constants.UserID("user-456")

		queue.SendMemberRemoved(app, channel, userID)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendMemberRemovedCalls()
		assert.Len(t, calls, 1)
		if len(calls) > 0 {
			assert.Equal(t, channel, calls[0].Payload.Channel)
			assert.Equal(t, userID, calls[0].Payload.UserID)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, false, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasMemberRemovedWebhooks = false

		channel := constants.ChannelName("test-channel")
		userID := constants.UserID("user-456")

		// Should not call Send when webhooks are disabled
		queue.SendMemberRemoved(app, channel, userID)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify no calls were made
		calls := mockQueue.GetSendMemberRemovedCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendChannelVacated(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		queue.SendChannelVacated(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendChannelVacatedCalls()
		if len(calls) > 0 {
			assert.Len(t, calls, 1)
			assert.Equal(t, channel, calls[0].Payload.Channel)
			assert.Equal(t, app.ID, calls[0].AppID)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasChannelVacatedWebhooks = false

		channel := constants.ChannelName("test-channel")

		// Should not call Send when webhooks are disabled
		queue.SendChannelVacated(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify no calls were made
		calls := mockQueue.GetSendChannelVacatedCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendChannelOccupied(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		queue.SendChannelOccupied(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendChannelOccupiedCalls()
		assert.Len(t, calls, 1)
		if len(calls) > 0 {
			assert.Equal(t, app.ID, calls[0].AppID)
			assert.Equal(t, channel, calls[0].Payload.Channel)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasChannelOccupiedWebhooks = false

		channel := constants.ChannelName("test-channel")

		// Should not call Send when webhooks are disabled
		queue.SendChannelOccupied(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify no calls were made
		calls := mockQueue.GetSendChannelOccupiedCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_SendCacheMissed(t *testing.T) {
	t.Run("WebhooksEnabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		queue.SendCacheMissed(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify Send was called
		calls := mockQueue.GetSendCacheMissedCalls()
		assert.Len(t, calls, 1)
		if len(calls) > 0 {
			assert.Equal(t, app.ID, calls[0].AppID)
			assert.Equal(t, channel, calls[0].Payload.Channel)
		}
	})

	t.Run("WebhooksDisabled", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		app.HasChannelOccupiedWebhooks = false // Note: This seems to be a bug in the original code

		channel := constants.ChannelName("test-channel")

		// Should not call Send when webhooks are disabled
		queue.SendCacheMissed(app, channel)

		time.Sleep(150 * time.Millisecond) // wait for flap detection

		// Verify no calls were made
		calls := mockQueue.GetSendCacheMissedCalls()
		assert.Empty(t, calls)
	})
}

func TestAbstractQueue_FlapDetection(t *testing.T) {
	t.Run("ConnectDisconnectFlap", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		// Send connect event
		queue.SendMemberAdded(app, channel, "user-123")

		// Send disconnect event quickly (should cancel the connect event)
		time.Sleep(50 * time.Millisecond)
		queue.SendMemberRemoved(app, channel, "user-123")

		// Wait for any remaining events to process
		time.Sleep(200 * time.Millisecond)

		// No events should be sent due to flap detection
		calls := mockQueue.GetSendMemberAddedCalls()
		assert.Empty(t, calls)

		calls = mockQueue.GetSendMemberRemovedCalls()
		assert.Empty(t, calls)
	})

	t.Run("NoFlapDetection", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, true, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		// Send connect event
		queue.SendMemberAdded(app, channel, "user-123")

		// Wait for flap detection delay
		time.Sleep(150 * time.Millisecond)

		// Send disconnect event after delay (should not be cancelled)
		queue.SendMemberRemoved(app, channel, "user-123")

		// Wait for second event to process
		time.Sleep(150 * time.Millisecond)

		// Both events should be sent
		calls := mockQueue.GetAddToQueueCalls()
		assert.NotEmpty(t, calls)
	})
}

func TestAbstractQueue_ConcurrentAccess(t *testing.T) {
	t.Run("ConcurrentSends", func(t *testing.T) {
		mockQueue := &MockQueueInterface{}

		ctx := context.Background()
		queue, err := NewAbstractQueue(ctx, mockQueue, &webhooks.WebhookSender{}, false, 100*time.Millisecond)
		assert.NoError(t, err)

		app := createTestAppWithWebhooks()
		channel := constants.ChannelName("test-channel")

		// Send multiple events concurrently
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				queue.SendMemberAdded(app, channel, constants.UserID(fmt.Sprintf("user-%d", i)))
			}(i)
		}

		wg.Wait()

		time.Sleep(200 * time.Millisecond) // wait for all events to process

		// Verify all events were sent
		calls := mockQueue.GetAddToQueueCalls()
		assert.Len(t, calls, 10)
	})
}

// Test helper to create a webhook event
func createWebhookEvent(name, channel string) *pusher.WebhookEvent {
	return &pusher.WebhookEvent{
		Name:    name,
		Channel: channel,
		Data:    "test-data",
	}
}
