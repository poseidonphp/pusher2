package internal

import (
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
)

type AdapterInterface interface {
	Init() error

	GetNamespace(appID constants.AppID) (*Namespace, error)
	GetNamespaces() (map[constants.AppID]*Namespace, error)

	AddSocket(appID constants.AppID, ws *WebSocket) error
	RemoveSocket(appID constants.AppID, wsID constants.SocketID) error

	AddToChannel(appID constants.AppID, channel constants.ChannelName, ws *WebSocket) (int64, error)
	RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64

	// Send sends a message to a Channel on a namespace
	Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error

	TerminateUserConnections(appID constants.AppID, userID string)
	Disconnect()
	ClearNamespace(appID constants.AppID)
	ClearNamespaces()

	GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket
	GetSocketsCount(appID constants.AppID, onlyLocal bool) int64
	GetChannels(appID constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID
	GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64
	GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket
	GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int64
	GetChannelMembers(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[string]*pusherClient.MemberData
	GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int
	GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64

	IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool
	AddUser(appID constants.AppID, ws *WebSocket) error
	RemoveUser(appID constants.AppID, ws *WebSocket) error
	GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error)
}
