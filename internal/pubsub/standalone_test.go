package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStandalonePubSub_Init(t *testing.T) {
	manager := &StandAlonePubSubManager{}

	err := manager.Init()
	assert.NoError(t, err)
	//assert.NotNil(t, manager.Channels)
	//assert.NotNil(t, manager.mutex)
}

func TestStandalonePubSub_getKeyName(t *testing.T) {
	manager := &StandAlonePubSubManager{
		PubSubCore: PubSubCore{
			keyPrefix: "test-prefix",
		},
	}

	key := manager.getKeyName("channel-name")
	assert.Equal(t, "test-prefix:channel-name", key)

	// Test without prefix
	manager.keyPrefix = ""
	key = manager.getKeyName("channel-name")
	assert.Equal(t, "channel-name", key)
}

//func TestStandalonePubSub_Subscribe(t *testing.T) {
//	manager := &StandAlonePubSubManager{}
//	err := manager.Init()
//	assert.NoError(t, err)
//
//	// Create receive channel
//	receiveChan := make(chan ServerMessage, 10)
//
//	// Subscribe to a channel
//	go manager.Subscribe("test-channel", receiveChan)
//
//	// Verify subscription was registered
//	manager.mutex.Lock()
//	channelKey := manager.getKeyName("test-channel")
//	subscribers, exists := manager.Channels[channelKey]
//	manager.mutex.Unlock()
//
//	assert.True(t, exists)
//	assert.Len(t, subscribers, 1)
//}

func TestStandalonePubSub_Publish(t *testing.T) {
	// test that multiple subscribers receive the same message
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	// Create multiple receive channels
	receiver1 := make(chan ServerMessage, 10)
	receiver2 := make(chan ServerMessage, 10)

	// Subscribe both to the same channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready1 := make(chan struct{})
	ready2 := make(chan struct{})

	go manager.SubscribeWithNotify(ctx, "test-channel", receiver1, ready1)
	go manager.SubscribeWithNotify(ctx, "test-channel", receiver2, ready2)

	<-ready1
	<-ready2

	// Create test message
	message := ServerMessage{
		NodeID:  "node1",
		Event:   "test-event",
		Payload: []byte(`{"test":"data"}`),
	}

	// Publish message
	err = manager.Publish(ctx, "test-channel", message)
	assert.NoError(t, err)

	// Verify both receivers got the message
	waitForMessage := func(ch <-chan ServerMessage) ServerMessage {
		select {
		case msg := <-ch:
			return msg
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for message")
			return ServerMessage{}
		}
	}

	msg1 := waitForMessage(receiver1)
	msg2 := waitForMessage(receiver2)

	assert.Equal(t, message, msg1)
	assert.Equal(t, message, msg2)
}

func TestStandalonePubSub_Publish_MultipleChannels(t *testing.T) {
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	// Create receive channels for different channels
	receiver1 := make(chan ServerMessage, 10)
	receiver2 := make(chan ServerMessage, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Subscribe to different channels
	ready1 := make(chan struct{})
	ready2 := make(chan struct{})

	go manager.SubscribeWithNotify(ctx, "channel1", receiver1, ready1)
	go manager.SubscribeWithNotify(ctx, "channel2", receiver2, ready2)

	<-ready1
	<-ready2

	// Create test messages
	message1 := ServerMessage{
		NodeID:  "node1",
		Event:   "event1",
		Payload: []byte(`{"channel":"1"}`),
	}

	message2 := ServerMessage{
		NodeID:  "node1",
		Event:   "event2",
		Payload: []byte(`{"channel":"2"}`),
	}

	// Publish messages to respective channels
	err = manager.Publish(ctx, "channel1", message1)
	assert.NoError(t, err)

	err = manager.Publish(ctx, "channel2", message2)
	assert.NoError(t, err)

	// Check messages were delivered correctly
	select {
	case msg := <-receiver1:
		assert.Equal(t, message1, msg)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for message on channel1")
	}

	select {
	case msg := <-receiver2:
		assert.Equal(t, message2, msg)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for message on channel2")
	}

	// Verify no cross-delivery
	select {
	case <-receiver1:
		t.Fatal("Received unexpected message on channel1")
	case <-receiver2:
		t.Fatal("Received unexpected message on channel2")
	case <-time.After(100 * time.Millisecond):
		// This is expected - no more messages should arrive
	}
}

func TestStandalonePubSub_Publish_CancelledContext(t *testing.T) {
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	// Create a receive channel
	receiver := make(chan ServerMessage, 10)

	ctx, cancel := context.WithCancel(context.Background())

	// Subscribe to a channel
	go manager.Subscribe(ctx, "test-channel", receiver)

	// Create test message
	message := ServerMessage{
		NodeID:  "node1",
		Event:   "test-event",
		Payload: []byte(`{"test":"data"}`),
	}

	// Create cancelled context
	cancel() // Cancel before publish

	// Publish with cancelled context
	err = manager.Publish(ctx, "test-channel", message)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Verify no message was delivered
	select {
	case <-receiver:
		t.Fatal("Message was delivered despite cancelled context")
	case <-time.After(100 * time.Millisecond):
		// This is expected - publish should fail with cancelled context
	}
}

func TestStandalonePubSub_UnsubscribeOnChannelClose(t *testing.T) {
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a receive channel and subscribe
	receiver := make(chan ServerMessage, 10)

	ready := make(chan struct{})
	go manager.SubscribeWithNotify(ctx, "test-channel", receiver, ready)
	<-ready

	// Verify subscription exists
	manager.mutex.Lock()
	channelKey := manager.getKeyName("test-channel")
	subscribers, exists := manager.subscriptions[channelKey]
	manager.mutex.Unlock()
	assert.True(t, exists)
	assert.Len(t, subscribers, 1)

	// Close the channel
	close(receiver)

	// Publish a message to trigger cleanup
	message := ServerMessage{
		NodeID:  "node1",
		Event:   "test-event",
		Payload: []byte(`{"test":"data"}`),
	}

	err = manager.Publish(ctx, "test-channel", message)
	assert.NoError(t, err)

	// Give some time for the automatic cleanup to happen
	time.Sleep(50 * time.Millisecond)

	// Verify closed channel was removed from subscriptions
	manager.mutex.Lock()
	subscribers, exists = manager.subscriptions[channelKey]
	manager.mutex.Unlock()

	assert.True(t, exists)       // The channel still exists in the map
	assert.Empty(t, subscribers) // But it has no subscribers
}

func TestStandalonePubSub_NoSubscribers(t *testing.T) {
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	// Publish to a channel with no subscribers (should not error)
	message := ServerMessage{
		NodeID:  "node1",
		Event:   "test-event",
		Payload: []byte(`{"test":"data"}`),
	}

	ctx := context.Background()
	err = manager.Publish(ctx, "nonexistent-channel", message)
	assert.NoError(t, err)
}

func TestStandalonePubSub_MultipleSubscriptions(t *testing.T) {
	manager := &StandAlonePubSubManager{}
	err := manager.Init()
	assert.NoError(t, err)

	// Create multiple subscriptions from the same receiver to different channels
	receiver := make(chan ServerMessage, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready1 := make(chan struct{})
	ready2 := make(chan struct{})

	go manager.SubscribeWithNotify(ctx, "channel1", receiver, ready1)
	go manager.SubscribeWithNotify(ctx, "channel2", receiver, ready2)

	<-ready1
	<-ready2

	// Publish to both channels
	message1 := ServerMessage{
		NodeID:  "node1",
		Event:   "event1",
		Payload: []byte(`{"channel":"1"}`),
	}

	message2 := ServerMessage{
		NodeID:  "node1",
		Event:   "event2",
		Payload: []byte(`{"channel":"2"}`),
	}

	err = manager.Publish(ctx, "channel1", message1)
	assert.NoError(t, err)

	err = manager.Publish(ctx, "channel2", message2)
	assert.NoError(t, err)

	// Verify both messages were received
	receivedCount := 0
	for i := 0; i < 2; i++ {
		select {
		case msg := <-receiver:
			receivedCount++
			assert.Contains(t, []ServerMessage{message1, message2}, msg)
		case <-time.After(time.Second):
			t.Fatal("Timed out waiting for message")
		}
	}

	assert.Equal(t, 2, receivedCount)
}
