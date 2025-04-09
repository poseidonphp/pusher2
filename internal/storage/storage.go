package storage

import (
	"encoding/json"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"
	"pusher/internal/constants"
	"pusher/internal/dispatcher"
	"pusher/internal/payloads"
	"pusher/internal/pubsub"
	"pusher/log"
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
	Start()

	// AddNewNode adds a node to the storage list of nodes
	AddNewNode(nodeID constants.NodeID) error

	// RemoveNode removes a node from the storage list of nodes
	RemoveNode(nodeID constants.NodeID) error

	//PurgeNodeData purges all data related to a node, after it has stopped sending heartbeats
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
	AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) error

	// GetChannelCount returns the count of subscribers for a channel
	GetChannelCount(channelName constants.ChannelName) int64

	AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error
	RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error
	GetPresenceData(channelName constants.ChannelName, currentUser pusherClient.MemberData) (presenceDataBytes []byte, usersSocketIDs []constants.SocketID, pErr error)
	GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error)

	SocketDidHeartbeat(nodeID constants.NodeID, socketID constants.SocketID, channels map[constants.ChannelName]constants.ChannelName) error

	// Cleanup is called by the hub periodically to clean up any stale data from dead nodes
	Cleanup()
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

func PresenceChannelUserIDs(channel constants.ChannelName) []string {
	subs, _, err := Manager.GetPresenceData(channel, pusherClient.MemberData{})
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

func GetPresenceSocketsForUserID(channelName constants.ChannelName, userID string) []constants.SocketID {
	_, userSockets, _ := Manager.GetPresenceData(channelName, pusherClient.MemberData{UserID: userID})

	return userSockets
}

func handleChannelCountChanges(channelName constants.ChannelName, newCount int64, modifiedCount int64) {
	log.Logger().Tracef("Channel %s count changed to %d (modified by %d)", channelName, newCount, modifiedCount)
	if newCount == 0 {
		// publish a vacated channel event
		log.Logger().Debugf("Channel %s is now vacated", channelName)
		event := pusherClient.WebhookEvent{
			Name:    string(constants.WebHookChannelVacated),
			Channel: string(channelName),
		}
		dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channelName), dispatcher.Disconnect, event)
	} else if newCount == modifiedCount {
		// if the count is equal to the amount we added, publish a channel occupied event
		log.Logger().Debugf("Channel %s is now occupied", channelName)
		event := pusherClient.WebhookEvent{
			Name:    string(constants.WebHookChannelOccupied),
			Channel: string(channelName),
		}
		dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channelName), dispatcher.Connect, event)
	}

	// in addition to the above, we will also send a 'subscription_count' webhook
	// TODO: Cannot implement until WebhookEvent is updated to include SubscriptionCount
	//event := pusherClient.WebhookEvent{
	//	Name:    string(constants.WebHookSubscriptionCount),
	//	Channel: string(channelName),
	//	SubscriptionCount:
	//}
	//dispatcher.DispatchBuffer.HandleEvent("channel:"+string(channelName), dispatcher.Connect, event)
}
