package constants

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConstantValues(t *testing.T) {
	// Test the constants
	assert.Equal(t, ChannelType("presence"), ChannelTypePresence)
	assert.Equal(t, ChannelType("private"), ChannelTypePrivate)
	assert.Equal(t, ChannelType("public"), ChannelTypePublic)
	assert.Equal(t, ChannelType("private-encrypted"), ChannelTypePrivateEncrypted)

	assert.Equal(t, "production", PRODUCTION)
	assert.Equal(t, "#server-to-user-", SocketRushServerToUserPrefix)
	assert.Equal(t, "pusher:error", PusherError)
	assert.Equal(t, "pusher:cache_miss", PusherCacheMiss)
	assert.Equal(t, "pusher:subscribe", PusherSubscribe)
	assert.Equal(t, "pusher:unsubscribe", PusherUnsubscribe)
	assert.Equal(t, "pusher:ping", PusherPing)
	assert.Equal(t, "pusher:pong", PusherPong)
	assert.Equal(t, "pusher:connection_established", PusherConnectionEstablished)
	assert.Equal(t, "pusher_internal:subscription_succeeded", PusherInternalSubscriptionSucceeded)
	assert.Equal(t, "pusher_internal:member_added", PusherInternalPresenceMemberAdded)
	assert.Equal(t, "pusher_internal:member_removed", PusherInternalPresenceMemberRemoved)
	assert.Equal(t, "pusher:subscription_error", PusherSubscriptionError)
	assert.Equal(t, "pusher:signin", PusherSignin)
	assert.Equal(t, "pusher:signin_success", PusherSigninSuccess)

	assert.Equal(t, WebHookEvent("member_added"), WebHookMemberAdded)
	assert.Equal(t, WebHookEvent("member_removed"), WebHookMemberRemoved)
	assert.Equal(t, WebHookEvent("channel_occupied"), WebHookChannelOccupied)
	assert.Equal(t, WebHookEvent("channel_vacated"), WebHookChannelVacated)
	assert.Equal(t, WebHookEvent("client_event"), WebHookClientEvent)
	assert.Equal(t, WebHookEvent("subscription_count"), WebHookSubscriptionCount)
	assert.Equal(t, WebHookEvent("cache_miss"), WebHookCacheMiss)

	assert.Equal(t, 10*time.Second, WriteWait)
	assert.Equal(t, 1024, MaxMessageSize)
	assert.Equal(t, [...]int{7}, SupportedProtocolVersions)
}
