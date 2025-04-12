package storage

import (
	"encoding/json"
	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/pubsub"
)

const (
	NodePingInterval                                = time.Duration(10) * time.Second
	ServerEventNewNodeJoined pubsub.ServerEventName = "new_node_joined"
	ServerEventNodeLeft      pubsub.ServerEventName = "node_left"
	ServerHeartbeat          pubsub.ServerEventName = "heartbeat"
)

type StorageContract interface {
	Init() error
	Start()

	// AddNewNode adds a node to the storage list of nodes
	AddNewNode(nodeID constants.NodeID) error

	// RemoveNode removes a node from the storage list of nodes
	RemoveNode(nodeID constants.NodeID) error

	// PurgeNodeData purges all data related to a node, after it has stopped sending heartbeats
	PurgeNodeData(nodeID constants.NodeID) error

	// GetAllNodes returns a list of all nodes
	GetAllNodes() ([]string, error)

	// SendNodeHeartbeat is used by the node to let storage know that it's still active
	SendNodeHeartbeat(nodeID constants.NodeID) *time.Time

	// AddChannel adds a channel to the list of channels
	AddChannel(channel constants.ChannelName) error

	// RemoveChannel removes a channel from the list of channels
	RemoveChannel(channel constants.ChannelName) error

	// Channels returns a list of all channels
	Channels() []constants.ChannelName

	// AdjustChannelCount is used to adjust the count of subscribers for a channel
	AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) (newCount int64, err error)

	// GetChannelCount returns the count of subscribers for a channel
	GetChannelCount(channelName constants.ChannelName) int64

	AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error
	RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error

	// GetPresenceData returns the presence data for a given channel and socketIDs for the current user (list of users and their data).
	GetPresenceData(channelName constants.ChannelName, currentUser pusherClient.MemberData) (presenceDataBytes []byte, usersSocketIDs []constants.SocketID, pErr error)
	GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error)

	SocketDidHeartbeat(nodeID constants.NodeID, socketID constants.SocketID, channels map[constants.ChannelName]constants.ChannelName) error

	// Cleanup is called by the hub periodically to clean up any stale data from dead nodes
	Cleanup()

	// GetPresenceSocketsForUserID(channelName constants.ChannelName, userID string) []constants.SocketID
	// handleChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64)
}

// Global Storage Functions:

func UnmarshalPresenceData(presenceData []byte) (payloads.PresenceData, error) {
	var data map[string]payloads.PresenceData
	err := json.Unmarshal(presenceData, &data)
	if err != nil {
		return payloads.PresenceData{}, err
	}

	return data["presence"], nil
}

func PresenceChannelUserIDs(storageManager StorageContract, channel constants.ChannelName) []string {
	subs, _, err := storageManager.GetPresenceData(channel, pusherClient.MemberData{})
	if err != nil {
		return nil
	}

	var userIDs []string

	unmarshalled, _ := UnmarshalPresenceData(subs)

	for _, userID := range unmarshalled.IDs {
		if userID != "" && !funk.Contains(userIDs, userID) {
			userIDs = append(userIDs, userID)
		}
	}
	return userIDs
}

func GetPresenceSocketsForUserID(storageManager StorageContract, channelName constants.ChannelName, userID string) []constants.SocketID {
	_, userSockets, _ := storageManager.GetPresenceData(channelName, pusherClient.MemberData{UserID: userID})

	return userSockets
}
