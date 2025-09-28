package constants

import "time"

type ChannelName = string // allow alias as string

type AppID = string // allow alias as string

type UserID = string // allow alias as string

type SocketID = string

type ChannelType string

type WebHookEvent string

const (
	ChannelTypePresence         ChannelType = "presence"
	ChannelTypePrivate          ChannelType = "private"
	ChannelTypePrivateEncrypted ChannelType = "private-encrypted"
	ChannelTypePublic           ChannelType = "public"
)

const (
	// PRODUCTION env
	PRODUCTION = "production"

	SocketRushServerToUserPrefix = "#server-to-user-"

	PusherError                         = "pusher:error"
	PusherCacheMiss                     = "pusher:cache_miss"
	PusherSubscribe                     = "pusher:subscribe"
	PusherUnsubscribe                   = "pusher:unsubscribe"
	PusherPing                          = "pusher:ping"
	PusherPong                          = "pusher:pong"
	PusherConnectionEstablished         = "pusher:connection_established"
	PusherInternalSubscriptionSucceeded = "pusher_internal:subscription_succeeded"
	PusherInternalPresenceMemberAdded   = "pusher_internal:member_added"
	PusherInternalPresenceMemberRemoved = "pusher_internal:member_removed"
	PusherSubscriptionError             = "pusher:subscription_error"
	PusherSignin                        = "pusher:signin"
	PusherSigninSuccess                 = "pusher:signin_success"
	// WEBHOOKS

	WebHookMemberAdded       WebHookEvent = "member_added"
	WebHookMemberRemoved     WebHookEvent = "member_removed"
	WebHookChannelOccupied   WebHookEvent = "channel_occupied"
	WebHookChannelVacated    WebHookEvent = "channel_vacated"
	WebHookClientEvent       WebHookEvent = "client_event"
	WebHookSubscriptionCount WebHookEvent = "subscription_count"
	WebHookCacheMiss         WebHookEvent = "cache_miss"

	WriteWait      = 10 * time.Second // Time allowed to write a message to the peer.
	MaxMessageSize = 20480            // Maximum message size allowed from peer.
	// MaxEventSizeKb = 10               // Maximum event size allowed from peer. 10KB
)

var SupportedProtocolVersions = [...]int{7}
