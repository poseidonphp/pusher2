package constants

import "time"

type ChannelName string

type NodeID string

type SocketID string

type ChannelType string

const (
	ChannelTypePresence         ChannelType = "presence"
	ChannelTypePrivate          ChannelType = "private"
	ChannelTypePrivateEncrypted ChannelType = "private-encrypted"
	ChannelTypePublic           ChannelType = "public"
)

const (
	// DEVELOPMENT env
	DEVELOPMENT = "development"
	// TEST env
	TEST = "test"
	// PRODUCTION env
	PRODUCTION = "production"

	PresenceChannelUserLimit             = 100
	MaxPresenceUserIDLength              = 128
	SoketRushInternalChannel             = "socket_rush_internal"
	SocketRushEventPresenceMemberAdded   = "presence-member-added"
	SocketRushEventPresenceMemberRemoved = "presence-member-removed"
	SocketRushEventChannelEvent          = "channel-event"
	SocketRushEventCleanerPromote        = "cleaner-promote"

	//Presence                    = "presence"
	//Private                     = "private"
	//Client                      = "client"
	PusherError                         = "pusher:error"
	PusherSubscribe                     = "pusher:subscribe"
	PusherUnsubscribe                   = "pusher:unsubscribe"
	PusherPing                          = "pusher:ping"
	PusherPong                          = "pusher:pong"
	PusherConnectionEstablished         = "pusher:connection_established"
	PusherSubscriptionSucceeded         = "pusher:subscription_succeeded" // send with list of presence users
	PusherInternalSubscriptionSucceeded = "pusher_internal:subscription_succeeded"
	PusherInternalPresenceMemberAdded   = "pusher_internal:member_added"
	PusherInternalPresenceMemberRemoved = "pusher_internal:member_removed"
	PusherChannelSetKey                 = "pusher:channels"
	PusherUserSetKey                    = "pusher:users"

	WebHookMemberAdded     = "member_added"
	WebHookMemberRemoved   = "member_removed"
	WebHookChannelOccupied = "channel_occupied"
	WebHookChannelVacated  = "channel_vacated"
	WebHookClientEvent     = "client_event"

	WriteWait      = 10 * time.Second // Time allowed to write a message to the peer.
	MaxMessageSize = 1024             // Maximum message size allowed from peer.
	MaxEventSizeKb = 10               // Maximum event size allowed from peer. 10KB
)

var SupportedProtocolVersions = [...]int{7}

func MaxEventSizeBytes() int {
	return MaxEventSizeKb * 1024
}
