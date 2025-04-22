package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

type Channel struct {
	App          *apps.App
	Name         constants.ChannelName
	Connections  map[constants.SocketID]bool
	Type         constants.ChannelType
	RequiresAuth bool
	IsCache      bool
	IsEncrypted  bool
}

type ChannelJoinResponse struct {
	Success            bool
	ErrorCode          util.ErrorCode
	Message            string
	Type               string
	ChannelConnections int64
	Member             *pusherClient.MemberData
}

type ChannelLeaveResponse struct {
	Success              bool
	RemainingConnections int64
	Member               *pusherClient.MemberData
}

func CreateChannelFromString(app *apps.App, channelName constants.ChannelName) *Channel {
	channel := &Channel{
		App:          app,
		Name:         channelName,
		Connections:  make(map[constants.SocketID]bool),
		Type:         constants.ChannelTypePrivate,
		RequiresAuth: false,
		IsCache:      false,
		IsEncrypted:  false,
	}

	if util.IsPresenceChannel(channel.Name) {
		channel.Type = constants.ChannelTypePresence
		channel.RequiresAuth = true
		if strings.HasPrefix(string(channel.Name), "presence-cache-") {
			channel.IsCache = true
		}
	} else if util.IsPrivateEncryptedChannel(channel.Name) {
		channel.Type = constants.ChannelTypePrivateEncrypted
		channel.RequiresAuth = true
		channel.IsEncrypted = true
		if strings.HasPrefix(string(channel.Name), "private-encrypted-cache-") {
			channel.IsCache = true
		}
	} else if util.IsPrivateChannel(channel.Name) {
		channel.Type = constants.ChannelTypePrivate
		channel.RequiresAuth = true
		if strings.HasPrefix(string(channel.Name), "private-cache-") {
			channel.IsCache = true
		}
	} else {
		channel.Type = constants.ChannelTypePublic
		channel.RequiresAuth = false
		if strings.HasPrefix(string(channel.Name), "cache-") {
			channel.IsCache = true
		}
	}
	log.Logger().Tracef("👥 Created new Channel: %s (type: %s / cache:%t)", channel.Name, channel.Type, channel.IsCache)

	return channel
}

func (c *Channel) Join(adapter AdapterInterface, ws *WebSocket, message payloads.SubscribePayload) *ChannelJoinResponse {
	if c.RequiresAuth {
		if !util.ValidateChannelAuth(message.Data.Auth, ws.ID, c.Name, message.Data.ChannelData) {
			return &ChannelJoinResponse{
				ErrorCode: util.ErrCodeSubscriptionAccessDenied,
				Message:   "Invalid signature",
				Type:      "AuthError",
			}
		}
	}

	var resp *ChannelJoinResponse

	switch c.Type {
	case constants.ChannelTypePresence:
		resp = c.joinPresenceChannel(adapter, ws, message)
	default:
		resp = c.joinNonPresenceChannel(adapter, ws, message)
	}

	return resp
}

func (c *Channel) joinPresenceChannel(adapter AdapterInterface, ws *WebSocket, message payloads.SubscribePayload) *ChannelJoinResponse {
	// Get Channel members count
	memberCount := adapter.GetChannelMembersCount(c.App.ID, c.Name, false)
	log.Logger().Infof("Total channel members for %s: %d", c.Name, memberCount)
	if memberCount+1 > c.App.MaxPresenceMembersPerChannel {
		e := &ChannelJoinResponse{
			ErrorCode: util.ErrCodeOverCapacity,
			Message:   "The maximum number of members in this Channel has been reached",
			Type:      "LimitReached",
			Success:   false,
		}
		return e
	}

	var member *pusherClient.MemberData
	_ = json.Unmarshal([]byte(message.Data.ChannelData), &member)

	// Check member size in kb
	if float64(len(member.UserInfo)/1024) > float64(c.App.MaxPresenceMemberSizeInKb) {
		e := &ChannelJoinResponse{
			ErrorCode: util.ErrCodeClientEventRejected,
			Message:   fmt.Sprintf("The maximum size of the member data is %d KB", c.App.MaxPresenceMemberSizeInKb),
			Type:      "LimitReached",
			Success:   false,
		}
		return e
	}

	// join
	_, joinErr := adapter.AddToChannel(c.App.ID, c.Name, ws)
	connectionCount := adapter.GetChannelSocketsCount(c.App.ID, c.Name, false)

	if joinErr != nil {
		// TODO handle error
		log.Logger().Errorf("Error joining Channel: %s", joinErr)
	}
	response := &ChannelJoinResponse{
		Success:            true,
		ChannelConnections: connectionCount,
		Member:             member,
	}
	return response
}

func (c *Channel) joinNonPresenceChannel(adapter AdapterInterface, ws *WebSocket, message payloads.SubscribePayload) *ChannelJoinResponse {
	connections, joinErr := adapter.AddToChannel(c.App.ID, c.Name, ws)
	if joinErr != nil {
		log.Logger().Errorf("Error joining Channel: %s", joinErr)
	}
	response := &ChannelJoinResponse{
		Success:            true,
		ChannelConnections: connections,
	}
	return response
}

func (c *Channel) Leave(adapter AdapterInterface, ws *WebSocket) *ChannelLeaveResponse {
	_ = adapter.RemoveFromChannel(c.App.ID, []constants.ChannelName{c.Name}, ws.ID)
	remainingConnections := adapter.GetChannelSocketsCount(c.App.ID, c.Name, false)

	log.Logger().Tracef("👥 Socket %s left Channel %s, remaining connections: %d", ws.ID, c.Name, remainingConnections)

	resp := &ChannelLeaveResponse{
		Success:              true,
		RemainingConnections: remainingConnections,
	}

	if c.Type == constants.ChannelTypePresence {
		// add member info
		member := ws.getPresenceDataForChannel(c.Name)
		if member != nil {
			resp.Member = member
		}
	}

	return resp
}

type ChannelEvent struct {
	Event    string                `json:"event"`
	Channel  constants.ChannelName `json:"Channel"`
	Data     string                `json:"data"`
	UserID   string                `json:"user_id,omitempty"`   // optional, present only if this is a `client event` on a `presence Channel`
	SocketID constants.SocketID    `json:"socket_id,omitempty"` // optional, skips the event from being sent to this socket
}

func (ce *ChannelEvent) DataToJson() ([]byte, error) {
	d, er := json.Marshal(ce.Data)
	if er != nil {
		log.Logger().Errorf("Error marshalling ChannelEvent data: %s", er)
		return nil, er
	}
	return d, nil
}

func (ce *ChannelEvent) ToJSON() []byte {
	b, err := json.Marshal(ce)
	if err != nil {
		log.Logger().Errorf("Error marshalling ChannelEvent: %s", err)
	}
	return b
}

// MemberRemovedData ...
type MemberRemovedData struct {
	UserID string `json:"user_id"`
}

func (mrd *MemberRemovedData) ToString() string {
	b, err := json.Marshal(mrd)
	if err != nil {
		log.Logger().Errorf("Error marshalling MemberRemovedData: %s", err)
	}
	return string(b)
}

func (c *Channel) addSocketID(socketID constants.SocketID) {
	mutex := sync.Mutex{}
	mutex.Lock()
	defer mutex.Unlock()
	c.Connections[socketID] = true
}

func (c *Channel) getOrCreateHubChannel() {

}
