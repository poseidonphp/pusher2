package queues

import (
	"context"
	"github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
	"pusher/internal/constants"
	"sync"
	"testing"
	"time"
)

// MockWebhookManager implements webhooks.WebhookContract for testing
type MockWebhookManager struct {
	mutex        sync.Mutex
	SentWebhooks []pusher.Webhook
	ShouldError  bool
}

func (m *MockWebhookManager) Send(webhook pusher.Webhook) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.SentWebhooks = append(m.SentWebhooks, webhook)

	if m.ShouldError {
		return assert.AnError
	}
	return nil
}

func TestSyncDispatcher_Init(t *testing.T) {
	dispatcher := &SyncQueue{}

	err := dispatcher.Init()
	assert.NoError(t, err)
	assert.NotNil(t, dispatcher.incomingMessages, "Channel should be initialized")
	assert.Equal(t, 100, cap(dispatcher.incomingMessages), "Channel should have capacity of 100")
}

func TestSyncDispatcher_Dispatch(t *testing.T) {
	dispatcher := &SyncQueue{}
	err := dispatcher.Init()
	assert.NoError(t, err)

	// Create test event
	event := pusher.WebhookEvent{
		Name:    "test-event",
		Channel: "test-channel",
		Data:    "test-data",
	}

	// Dispatch in a goroutine to avoid blocking on channel
	go dispatcher.Dispatch(event)

	// Wait for event to be received
selectLoop:
	select {
	case receivedEvent := <-dispatcher.incomingMessages:
		assert.Equal(t, event, receivedEvent)
		break selectLoop
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for event")
	}
}

func TestSyncDispatcher_Dispatch_PrivateEncrypted(t *testing.T) {
	dispatcher := &SyncQueue{}
	err := dispatcher.Init()
	assert.NoError(t, err)

	// Create test event for encrypted channel
	event := pusher.WebhookEvent{
		Name:    "test-event",
		Channel: "private-encrypted-channel",
		Data:    "test-data",
	}

	// Dispatch in a goroutine to avoid blocking
	go dispatcher.Dispatch(event)

	// Wait for event to be received
selectLoop:
	select {
	case receivedEvent := <-dispatcher.incomingMessages:
		assert.Equal(t, event, receivedEvent)
		assert.Equal(t, "private-encrypted-channel", receivedEvent.Channel)
		break selectLoop
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for event")
	}
}

func TestSyncDispatcher_ListenForEvents(t *testing.T) {
	mockWebhook := &MockWebhookManager{
		SentWebhooks: []pusher.Webhook{},
	}

	dispatcher := &SyncQueue{
		WebhookManager: mockWebhook,
	}
	err := dispatcher.Init()
	assert.NoError(t, err)

	// Create context with cancel function
	ctx, cancel := context.WithCancel(context.Background())

	// Start listening in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatcher.ListenForEvents(ctx)
	}()

	// Wait a bit for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Test events
	events := []pusher.WebhookEvent{
		{
			Name:    "event1",
			Channel: "channel1",
			Data:    "data1",
		},
		{
			Name:    "event2",
			Channel: "channel2",
			Data:    "data2",
		},
	}

	// Dispatch events
	for _, event := range events {
		dispatcher.Dispatch(event)
	}

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop listener
	cancel()
	wg.Wait()

	// Check that events were sent to webhook manager
	assert.Equal(t, len(events), len(mockWebhook.SentWebhooks))

	for i, webhook := range mockWebhook.SentWebhooks {
		assert.Len(t, webhook.Events, 1)
		assert.Equal(t, events[i].Name, webhook.Events[0].Name)
		assert.Equal(t, events[i].Channel, webhook.Events[0].Channel)
		assert.Equal(t, events[i].Data, webhook.Events[0].Data)
	}
}

func TestSyncDispatcher_ListenForEvents_WebhookError(t *testing.T) {
	mockWebhook := &MockWebhookManager{
		SentWebhooks: []pusher.Webhook{},
		ShouldError:  true,
	}

	dispatcher := &SyncQueue{
		WebhookManager: mockWebhook,
	}
	err := dispatcher.Init()
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatcher.ListenForEvents(ctx)
	}()

	// Wait a bit for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Send an event that will trigger an error
	dispatcher.Dispatch(pusher.WebhookEvent{
		Name:    "error-event",
		Channel: "error-channel",
	})

	// Wait for event to be processed
	time.Sleep(100 * time.Millisecond)

	// The test passes if ListenForEvents doesn't crash on webhook error
	cancel()
	wg.Wait()

	assert.Equal(t, 1, len(mockWebhook.SentWebhooks))
}

func TestSyncDispatcher_InitFlapDetection(t *testing.T) {
	dispatcher := &SyncQueue{}

	// Test with webhook enabled
	dispatcher.InitFlapDetection(true, dispatcher, 1)
	assert.NotNil(t, dispatcher.FlapDetector)
	assert.True(t, dispatcher.FlapDetector.WebhookEnabled)
	assert.Equal(t, dispatcher, dispatcher.FlapDetector.dispatcher)

	// Test with webhook disabled
	dispatcher = &SyncQueue{}
	dispatcher.InitFlapDetection(false, dispatcher, 1)
	assert.NotNil(t, dispatcher.FlapDetector)
	assert.False(t, dispatcher.FlapDetector.WebhookEnabled)
}

func TestSyncDispatcher_SendChannelCountChanges(t *testing.T) {
	mockWebhook := &MockWebhookManager{}

	dispatcher := &SyncQueue{
		WebhookManager: mockWebhook,
	}

	// Initialize flap detector
	dispatcher.InitFlapDetection(true, dispatcher, 1)
	err := dispatcher.Init()
	assert.NoError(t, err)

	// Start listening in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatcher.ListenForEvents(ctx)
	}()

	// Test channel occupied event
	dispatcher.SendChannelCountChanges(constants.ChannelName("test-channel"), 5, 5)

	// Wait for event to be processed (including the 1-second flap detection delay)
	time.Sleep(1500 * time.Millisecond)

	// Check occupied event was sent
	assert.Equal(t, 1, len(mockWebhook.SentWebhooks))
	assert.Equal(t, string(constants.WebHookChannelOccupied), mockWebhook.SentWebhooks[0].Events[0].Name)
	assert.Equal(t, "test-channel", mockWebhook.SentWebhooks[0].Events[0].Channel)

	// Clear sent webhooks
	mockWebhook.SentWebhooks = []pusher.Webhook{}

	// Test channel vacated event
	dispatcher.SendChannelCountChanges(constants.ChannelName("test-channel"), 0, -5)

	// Wait for event to be processed
	time.Sleep(1500 * time.Millisecond)

	// Check vacated event was sent
	assert.Equal(t, 1, len(mockWebhook.SentWebhooks))
	assert.Equal(t, string(constants.WebHookChannelVacated), mockWebhook.SentWebhooks[0].Events[0].Name)
	assert.Equal(t, "test-channel", mockWebhook.SentWebhooks[0].Events[0].Channel)
}
