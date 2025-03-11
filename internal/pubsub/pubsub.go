package pubsub

import (
	"fmt"
	"pusher/internal/constants"
)

var PubSubManager PubSubManagerContract

type PubSubManagerContract interface {
	Subscribe(channelName constants.ChannelName, receiveChannel chan<- ServerMessage)
	Publish(channelName constants.ChannelName, message ServerMessage) error
}

type ServerEventName string

type ServerMessage struct {
	NodeID   constants.NodeID   `json:"node_id"`
	Event    ServerEventName    `json:"event"`
	Payload  []byte             `json:"payload"`
	SocketID constants.SocketID `json:"socket_id"` // optional - can be used to exclude the socket from receiving the event
}

type PubSubCore struct {
	keyPrefix string
}

func (c *PubSubCore) getKeyName(key string) string {
	if c.keyPrefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", c.keyPrefix, key)
}
