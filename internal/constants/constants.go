package constants

import "time"

type ChannelName string

type NodeID string

type SocketID string

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

	MaxPresenceUserIDLength       = 128
	SocketRushInternalChannel     = "socket_rush_internal"
	SocketRushEventChannelEvent   = "channel-event"
	SocketRushEventCleanerPromote = "cleaner-promote"

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

	// WEBHOOKS

	WebHookMemberAdded       WebHookEvent = "member_added"
	WebHookMemberRemoved     WebHookEvent = "member_removed"
	WebHookChannelOccupied   WebHookEvent = "channel_occupied"
	WebHookChannelVacated    WebHookEvent = "channel_vacated"
	WebHookClientEvent       WebHookEvent = "client_event"
	WebHookSubscriptionCount WebHookEvent = "subscription_count"
	WebHookCacheMiss         WebHookEvent = "cache_miss"

	WriteWait      = 10 * time.Second // Time allowed to write a message to the peer.
	MaxMessageSize = 1024             // Maximum message size allowed from peer.
	MaxEventSizeKb = 10               // Maximum event size allowed from peer. 10KB
)

var SupportedProtocolVersions = [...]int{7}
