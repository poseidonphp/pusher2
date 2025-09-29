package internal

import (
	"encoding/json"
	"fmt"
	"strings"

	"pusher/internal/apps"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

// Channel represents a communication channel within an app namespace.
// It contains metadata about the channel type, authentication requirements,
// caching behavior, and encryption settings. Channels are the primary
// mechanism for organizing WebSocket connections and enabling pub/sub messaging.
type Channel struct {
	App          *apps.App                   // The app this channel belongs to
	Name         constants.ChannelName       // The unique name of the channel
	Connections  map[constants.SocketID]bool // Map of socket IDs connected to this channel
	Type         constants.ChannelType       // Channel type (public, private, presence, etc.)
	RequiresAuth bool                        // Whether this channel requires authentication
	IsCache      bool                        // Whether this channel supports caching
	IsEncrypted  bool                        // Whether this channel uses encryption
}

func (c *Channel) String() string {
	return string(c.Name)
}

// ChannelJoinResponse represents the result of a channel join operation.
// It contains success status, error information if the join failed,
// connection count, and member data for presence channels.
type ChannelJoinResponse struct {
	Success            bool                     // Whether the join operation was successful
	ErrorCode          util.ErrorCode           // Error code if the join failed
	Message            string                   // Human-readable error message
	Type               string                   // Error type for client handling
	ChannelConnections int64                    // Total number of connections in the channel
	Member             *pusherClient.MemberData // Member data for presence channels
}

// ChannelLeaveResponse represents the result of a channel leave operation.
// It contains success status, remaining connection count, and member data
// for presence channels to handle member removal events.
type ChannelLeaveResponse struct {
	Success              bool                     // Whether the leave operation was successful
	RemainingConnections int64                    // Number of connections remaining in the channel
	Member               *pusherClient.MemberData // Member data for presence channels
}

// CreateChannelFromString creates a new Channel instance from a channel name string.
// It analyzes the channel name to determine the channel type, authentication requirements,
// caching behavior, and encryption settings based on naming conventions.
// Channel types are determined by prefixes: "presence-", "private-", "private-encrypted-", etc.
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

	// Determine channel type and properties based on naming conventions
	if util.IsPresenceChannel(channel.Name) {
		// Presence channels track user information and require authentication
		channel.Type = constants.ChannelTypePresence
		channel.RequiresAuth = true
		if strings.HasPrefix(channel.Name, "presence-cache-") {
			channel.IsCache = true
		}
	} else if util.IsPrivateEncryptedChannel(channel.Name) {
		// Private encrypted channels require authentication and use encryption
		channel.Type = constants.ChannelTypePrivateEncrypted
		channel.RequiresAuth = true
		channel.IsEncrypted = true
		if strings.HasPrefix(channel.Name, "private-encrypted-cache-") {
			channel.IsCache = true
		}
	} else if util.IsPrivateChannel(channel.Name) {
		// Private channels require authentication but no encryption
		channel.Type = constants.ChannelTypePrivate
		channel.RequiresAuth = true
		if strings.HasPrefix(channel.Name, "private-cache-") {
			channel.IsCache = true
		}
	} else {
		// Public channels are open to all and require no authentication
		channel.Type = constants.ChannelTypePublic
		channel.RequiresAuth = false
		if strings.HasPrefix(channel.Name, "cache-") {
			channel.IsCache = true
		}
	}
	log.Logger().Tracef("👥 Created new Channel: %s (type: %s / cache:%t)", channel.Name, channel.Type, channel.IsCache)

	return channel
}

// Join handles the process of adding a WebSocket to a channel.
// It performs authentication validation if required, then delegates to the appropriate
// join method based on channel type (presence vs non-presence). This is the main
// entry point for all channel subscription logic.
func (c *Channel) Join(adapter AdapterInterface, ws *WebSocket, message payloads.SubscribePayload) *ChannelJoinResponse {
	if adapter == nil {
		return &ChannelJoinResponse{
			ErrorCode: util.ErroCodeInternalServerError,
			Message:   "No adapter provided to Join()",
			Type:      "InternalError",
			Success:   false,
		}
	}
	// Validate authentication if required for this channel type
	if c.RequiresAuth {
		if !util.ValidateChannelAuth(message.Data.Auth, c.App.Secret, ws.ID, c.Name, message.Data.ChannelData) {

			return &ChannelJoinResponse{
				ErrorCode: util.ErrCodeSubscriptionAccessDenied,
				Message:   "Invalid signature",
				Type:      "AuthError",
				Success:   false,
			}
		}
	}

	var resp *ChannelJoinResponse

	// Delegate to appropriate join method based on channel type
	switch c.Type {
	case constants.ChannelTypePresence:
		resp = c.joinPresenceChannel(adapter, ws, message)
	default:
		resp = c.joinNonPresenceChannel(adapter, ws, message)
	}

	return resp
}

// joinPresenceChannel handles the specific logic for joining presence channels.
// It validates member limits, checks member data size, and processes the member
// information for presence functionality. Presence channels track user information
// and provide member lists to connected clients.
func (c *Channel) joinPresenceChannel(adapter AdapterInterface, ws *WebSocket, message payloads.SubscribePayload) *ChannelJoinResponse {
	// Check if adding this member would exceed the channel's member limit
	if c.App.MaxPresenceMembersPerChannel > 0 {
		memberCount := adapter.GetChannelMembersCount(c.App.ID, c.Name, false)
		log.Logger().Debugf("Total channel members for %s: %d", c.Name, memberCount)
		if memberCount+1 > c.App.MaxPresenceMembersPerChannel {
			e := &ChannelJoinResponse{
				ErrorCode: util.ErrCodeOverCapacity,
				Message:   "The maximum number of members in this Channel has been reached",
				Type:      "LimitReached",
				Success:   false,
			}
			return e
		}
	}

	// Validate member data size against app limits
	if float64(len(message.Data.ChannelData)/1024) > float64(c.App.MaxPresenceMemberSizeInKb) {
		e := &ChannelJoinResponse{
			ErrorCode: util.ErrCodeClientEventRejected,
			Message:   fmt.Sprintf("The maximum size of the member data is %d KB", c.App.MaxPresenceMemberSizeInKb),
			Type:      "LimitReached",
			Success:   false,
		}
		return e
	}

	// Parse member data from the subscription payload
	var member *pusherClient.MemberData
	_ = json.Unmarshal([]byte(message.Data.ChannelData), &member)

	// Add the socket to the channel through the adapter
	_, joinErr := adapter.AddToChannel(c.App.ID, c.Name, ws)
	connectionCount := adapter.GetChannelSocketsCount(c.App.ID, c.Name, false)

	if joinErr != nil {
		e := &ChannelJoinResponse{
			ErrorCode: util.ErroCodeInternalServerError,
			Message:   fmt.Sprintf("Error joining Channel: %s", joinErr),
			Type:      "InternalError",
			Success:   false,
		}
		return e
	}

	// Return successful join response with member data
	response := &ChannelJoinResponse{
		Success:            true,
		ChannelConnections: connectionCount,
		Member:             member,
	}
	return response
}

// joinNonPresenceChannel handles the logic for joining regular channels (public, private, encrypted).
// It adds the WebSocket to the channel through the adapter and returns the connection count.
// This method is simpler than presence channel joining as it doesn't need to handle
// member data or user information.
func (c *Channel) joinNonPresenceChannel(adapter AdapterInterface, ws *WebSocket, _ payloads.SubscribePayload) *ChannelJoinResponse {
	// Add the socket to the channel and get the connection count
	connections, joinErr := adapter.AddToChannel(c.App.ID, c.Name, ws)
	if joinErr != nil {
		e := &ChannelJoinResponse{
			ErrorCode: util.ErroCodeInternalServerError,
			Message:   fmt.Sprintf("Error joining Channel: %s", joinErr),
			Type:      "InternalError",
			Success:   false,
		}
		return e
	}

	// Return successful join response with connection count
	response := &ChannelJoinResponse{
		Success:            true,
		ChannelConnections: connections,
	}
	return response
}

// Leave handles the process of removing a WebSocket from a channel.
// It removes the socket from the channel through the adapter, gets the remaining
// connection count, and for presence channels, includes member data for proper
// member removal event handling.
func (c *Channel) Leave(adapter AdapterInterface, ws *WebSocket) *ChannelLeaveResponse {
	// Remove the socket from the channel through the adapter
	_ = adapter.RemoveFromChannel(c.App.ID, []constants.ChannelName{c.Name}, ws.ID)
	remainingConnections := adapter.GetChannelSocketsCount(c.App.ID, c.Name, false)

	log.Logger().Tracef("👥 Socket %s left Channel %s, remaining connections: %d", ws.ID, c.Name, remainingConnections)

	// Create the leave response with remaining connection count
	resp := &ChannelLeaveResponse{
		Success:              true,
		RemainingConnections: remainingConnections,
	}

	// For presence channels, include member data for proper member removal handling
	if c.Type == constants.ChannelTypePresence {
		member := ws.getPresenceDataForChannel(c.Name)
		if member != nil {
			resp.Member = member
		}
	}

	return resp
}
