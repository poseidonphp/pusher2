package queues

import (
	"testing"
	"time"

	"github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
	"pusher/internal/apps"
	"pusher/internal/constants"
)

func TestFlapDetector_Init(t *testing.T) {
	fd := &FlapDetector{
		FlapEnabled:         true,
		flapWindowInSeconds: 2 * time.Second,
	}
	fd.Init()
	if fd.pending == nil {
		t.Error("Expected pending map to be initialized")
	}
	if fd.flapWindowInSeconds != 2*time.Second {
		t.Errorf("Expected flapWindowInSeconds to be 2 seconds, got %v", fd.flapWindowInSeconds)
	}

	fd2 := &FlapDetector{
		FlapEnabled: true,
	}
	fd2.Init()
	assert.Equal(t, 3*time.Second, fd2.flapWindowInSeconds, "Expected flapWindowInSeconds to be 3 when not set before init")

	fd3 := &FlapDetector{
		FlapEnabled: false,
	}
	fd3.Init()
	assert.Nil(t, fd3.pending, "Expected pending map to be nil when FlapEnabled is false")
}

func TestFlapDetector_CheckForFlapping(t *testing.T) {
	wasCalled := false
	dispatchFn := func(app *apps.App, webhookEvent *pusher.WebhookEvent) {
		wasCalled = true
	}

	// want to make sure the dispatchFn is called immediately if flap detection is disabled
	t.Run("Disabled", func(t *testing.T) {
		fd := &FlapDetector{
			FlapEnabled:         false,
			flapWindowInSeconds: 2 * time.Second,
		}
		app := &apps.App{}
		wasCalled = false

		webhookEvent := &pusher.WebhookEvent{Name: string(constants.WebHookMemberAdded)}
		fd.CheckForFlapping(app, "user1", Connect, webhookEvent, dispatchFn)
		assert.True(t, wasCalled, "Expected dispatchFn to be called immediately when flap detection is disabled")
	})

	t.Run("Enabled_Flapping", func(t *testing.T) {
		fd := &FlapDetector{
			FlapEnabled:         true,
			flapWindowInSeconds: 100 * time.Millisecond,
		}
		fd.Init()
		app := &apps.App{}
		wasCalled = false

		webhookEvent := &pusher.WebhookEvent{Name: string(constants.WebHookMemberAdded)}
		fd.CheckForFlapping(app, "user1", Connect, webhookEvent, dispatchFn)
		assert.False(t, wasCalled, "Expected dispatchFn not to be called immediately when flap detection is enabled")

		time.Sleep(50 * time.Millisecond)
		webhookEvent2 := &pusher.WebhookEvent{Name: string(constants.WebHookMemberRemoved)}

		fd.CheckForFlapping(app, "user1", Disconnect, webhookEvent2, dispatchFn)
		assert.False(t, wasCalled, "Expected dispatchFn not to be called on opposite event within flap window")

		time.Sleep(150 * time.Millisecond)
		assert.False(t, wasCalled, "This event should not have fired due to flap detection")
	})

	t.Run("Enabled_NonFlapping", func(t *testing.T) {
		fd := &FlapDetector{
			FlapEnabled:         true,
			flapWindowInSeconds: 100 * time.Millisecond,
		}
		fd.Init()
		app := &apps.App{}
		wasCalled = false

		webhookEvent := &pusher.WebhookEvent{Name: string(constants.WebHookMemberAdded)}
		fd.CheckForFlapping(app, "user2", Connect, webhookEvent, dispatchFn)
		assert.False(t, wasCalled, "Expected dispatchFn not to be called immediately when flap detection is enabled")

		time.Sleep(150 * time.Millisecond) // wait longer than flap window
		assert.True(t, wasCalled, "Expected dispatchFn to be called after flap window for non-flapping event")
	})

}
