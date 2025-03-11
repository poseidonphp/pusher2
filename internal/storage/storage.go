package storage

import (
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/internal/pubsub"
	"time"
)

var Manager Contract

const (
	NodePingInterval                                = time.Duration(10) * time.Second
	ServerEventNewNodeJoined pubsub.ServerEventName = "new_node_joined"
	ServerEventNodeLeft      pubsub.ServerEventName = "node_left"
	ServerHeartbeat          pubsub.ServerEventName = "heartbeat"
)

type Contract interface {
	AddNewNode(nodeID constants.NodeID) error
	RemoveNode(nodeID constants.NodeID) error
	PurgeNodeData(nodeID constants.NodeID) error
	GetAllNodes() ([]string, error)
	SendNodeHeartbeat(nodeID constants.NodeID)

	// AddChannel adds a channel to the list of channels
	AddChannel(channel constants.ChannelName) error
	// RemoveChannel removes a channel from the list of channels
	RemoveChannel(channel constants.ChannelName) error
	// Channels returns a list of all channels
	Channels() []constants.ChannelName
	AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) error
	GetChannelCount(channelName constants.ChannelName) int64

	AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error
	RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error
	GetPresenceData(channelName constants.ChannelName) ([]byte, error)
	GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error)

	SocketDidHeartbeat(nodeID constants.NodeID, socketID constants.SocketID, channels map[constants.ChannelName]constants.ChannelName) error

	// Cleanup is called by the hub periodically to clean up any stale data from dead nodes
	Cleanup()
}
