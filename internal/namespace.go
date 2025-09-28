package internal

import (
	"slices"
	"sync"
	"time"

	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

// Namespace represents a collection of channels, sockets, and users for a specific app.
// It manages the relationships between WebSocket connections, channels, and user sessions
// within a single application namespace. This is the core data structure for managing
// real-time communication state for each app in the system.
type Namespace struct {
	Channels map[constants.ChannelName][]constants.SocketID // list of Channel connections for the current app
	Sockets  map[constants.SocketID]*WebSocket              // list of sockets connected to the namespace
	Users    map[string][]constants.SocketID                // list of user id's and their associated socket ids
	mutex    sync.Mutex
}

// ============================================================================
// PUBLIC METHODS
// ============================================================================

// GetSockets returns all WebSocket connections in this namespace.
// This provides access to all connected sockets for the app.
func (n *Namespace) GetSockets() map[constants.SocketID]*WebSocket {
	return n.Sockets
}

// GetChannels returns all channels and their subscribed socket IDs in this namespace.
// This provides a complete view of all channel subscriptions for the app.
func (n *Namespace) GetChannels() map[constants.ChannelName][]constants.SocketID {
	return n.Channels
}

// GetChannelsWithSocketsCount returns a map of channel names to their connection counts.
// This is useful for monitoring and metrics collection, providing quick access to
// how many connections each channel has without needing to iterate through socket lists.
func (n *Namespace) GetChannelsWithSocketsCount() map[constants.ChannelName]int64 {
	channels := n.GetChannels()
	list := make(map[constants.ChannelName]int64)

	for channel, sockets := range channels {
		list[channel] = int64(len(sockets))
	}
	return list
}

// GetPresenceChannelsWithUsersCount returns a map of presence channel names to their user counts.
// This method specifically targets presence channels and counts unique users rather than
// total connections, which is important for presence channel functionality where multiple
// sockets can belong to the same user.
func (n *Namespace) GetPresenceChannelsWithUsersCount() map[constants.ChannelName]int64 {
	log.Logger().Tracef("called GetPresenceChannelsWithUsersCount() within namespace")
	channels := n.GetChannels()
	list := make(map[constants.ChannelName]int64)

	for channel := range channels {
		if util.IsPresenceChannel(channel) {
			members := n.GetChannelMembers(channel)
			list[channel] = int64(len(members))
		}
	}
	return list
}

// CompactMaps removes empty entries from the namespace maps to free up memory.
// This method is called periodically to clean up stale references and prevent
// memory leaks from disconnected sockets or empty channels. It creates completely
// new maps and slices with only the active entries, helping maintain optimal memory usage.
// This also rebuilds the underlying hash tables to shrink them if many entries were deleted.
func (n *Namespace) CompactMaps() {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Create new maps with conservative capacity estimates
	// Use a smaller initial capacity to force hash table shrinking if many entries were deleted
	activeChannels := 0
	activeSockets := 0
	activeUsers := 0

	// Count active entries first
	for _, sockets := range n.Channels {
		if len(sockets) > 0 {
			activeChannels++
		}
	}
	for _, socket := range n.Sockets {
		if socket != nil {
			activeSockets++
		}
	}
	for _, socketIDs := range n.Users {
		if len(socketIDs) > 0 {
			activeUsers++
		}
	}

	// Create new maps with conservative capacity (75% of active entries to force shrinking)
	newChannels := make(map[constants.ChannelName][]constants.SocketID, (activeChannels*3)/4)
	newSockets := make(map[constants.SocketID]*WebSocket, (activeSockets*3)/4)
	newUsers := make(map[string][]constants.SocketID, (activeUsers*3)/4)

	// Copy only non-empty channels and create new slices to free old array references
	for channel, sockets := range n.Channels {
		if len(sockets) > 0 {
			// Create a new slice with exact capacity to avoid holding references to old arrays
			newSockets := make([]constants.SocketID, len(sockets))
			copy(newSockets, sockets)
			newChannels[channel] = newSockets
		}
	}

	// Copy only non-nil sockets
	for socketID, socket := range n.Sockets {
		if socket != nil {
			newSockets[socketID] = socket
		}
	}

	// Copy only non-empty users and create new slices to free old array references
	for userID, socketIDs := range n.Users {
		if len(socketIDs) > 0 {
			// Create a new slice with exact capacity to avoid holding references to old arrays
			newSocketIDs := make([]constants.SocketID, len(socketIDs))
			copy(newSocketIDs, socketIDs)
			newUsers[userID] = newSocketIDs
		}
	}

	// Replace the old maps with new ones (this allows garbage collection of old maps and their underlying data)
	n.Channels = newChannels
	n.Sockets = newSockets
	n.Users = newUsers
}

// AddSocket adds a new WebSocket connection to the namespace.
// It checks if the socket already exists to prevent duplicates and returns
// false if the socket is already present. This ensures each socket ID
// appears only once in the namespace.
func (n *Namespace) AddSocket(ws *WebSocket) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Sockets[ws.ID]; ok {
		return false
	}
	n.Sockets[ws.ID] = ws
	return true
}

// RemoveSocket removes a WebSocket connection from the namespace and cleans up all references.
// It removes the socket from all channels, handles user cleanup if the socket was associated
// with a user, and ensures complete cleanup of the socket's presence in the namespace.
// This method is called when a WebSocket connection is closed or terminated.
func (n *Namespace) RemoveSocket(wsID constants.SocketID) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Get the socket and check if it exists
	socket, exists := n.Sockets[wsID]
	if !exists {
		return // Socket doesn't exist, nothing to clean up
	}

	// Remove from sockets map first
	n.Sockets[wsID] = nil
	delete(n.Sockets, wsID)

	// Handle user cleanup if this socket was associated with a user
	if socket.userID != "" {
		// Remove from user's socket list
		if userSockets, ok := n.Users[socket.userID]; ok {
			// Create a new slice without the websocketId to avoid keeping references to the original array
			newSockets := make([]constants.SocketID, 0, len(userSockets)-1)
			for _, id := range userSockets {
				if id != wsID {
					newSockets = append(newSockets, id)
				}
			}

			// Replace the user's slice with the new one
			n.Users[socket.userID] = newSockets

			// If user has no more sockets, remove the user entry
			if len(n.Users[socket.userID]) == 0 {
				delete(n.Users, socket.userID)
			}
		}
	}

	// Remove socket from all channels
	for channel := range n.Channels {
		if _, ok := n.Channels[channel]; ok {
			// Create a new slice without the websocketId to avoid keeping references to the original array
			originalSockets := n.Channels[channel]
			newSockets := make([]constants.SocketID, 0, len(originalSockets)-1)

			for _, id := range originalSockets {
				if id != wsID {
					newSockets = append(newSockets, id)
				}
			}

			// Replace the channel's slice with the new one
			n.Channels[channel] = newSockets

			// If channel is empty, delete it
			if len(n.Channels[channel]) == 0 {
				n.Channels[channel] = nil
				delete(n.Channels, channel)
			}
		}
	}
}

// AddToChannel subscribes a WebSocket to a specific channel within the namespace.
// It creates the channel if it doesn't exist and adds the socket to the channel's
// subscriber list. Returns the total number of connections in the channel after
// adding this socket, which is useful for monitoring channel occupancy.
func (n *Namespace) AddToChannel(ws *WebSocket, channel constants.ChannelName) int64 {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Channels[channel]; !ok {
		n.Channels[channel] = []constants.SocketID{}
	}
	n.Channels[channel] = append(n.Channels[channel], ws.ID)
	return int64(len(n.Channels[channel]))
}

// RemoveFromChannel removes a WebSocket from one or more channels within the namespace.
// It handles the removal logic, cleans up empty channels, and returns the number of
// remaining connections. If multiple channels are specified, it returns 0. If a single
// channel is specified, it returns the remaining connection count for that channel.
func (n *Namespace) RemoveFromChannel(websocketId constants.SocketID, channels []constants.ChannelName) int64 {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	remove := func(channel constants.ChannelName) int64 {
		if _, ok := n.Channels[channel]; ok {
			// Create a new slice without the websocketId to avoid keeping references to the original array
			originalSockets := n.Channels[channel]
			newSockets := make([]constants.SocketID, 0, len(originalSockets)-1)

			for _, id := range originalSockets {
				if id != websocketId {
					newSockets = append(newSockets, id)
				}
			}

			// Replace the channel's slice with the new one
			n.Channels[channel] = newSockets

			// if the Channel is empty, delete it
			if len(n.Channels[channel]) == 0 {
				n.Channels[channel] = nil
				delete(n.Channels, channel)
				return 0
			}
			return int64(len(n.Channels[channel]))
		}
		return 0
	}

	removeResults := make(map[constants.ChannelName]int64)
	for _, channel := range channels {
		if _, ok := n.Channels[channel]; ok {
			removeResults[channel] = remove(channel)
		}
	}

	if len(channels) > 1 {
		return 0
	} else if len(channels) == 1 {
		return removeResults[channels[0]]
	} else {
		return 0
	}
}

// GetChannelSockets returns all WebSocket connections subscribed to a specific channel.
// It creates a new map containing only the sockets that are currently subscribed to
// the specified channel, filtering out any sockets that may have been disconnected
// but not yet cleaned up from the channel list.
func (n *Namespace) GetChannelSockets(channelName constants.ChannelName) map[constants.SocketID]*WebSocket {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	returnVal := make(map[constants.SocketID]*WebSocket)

	if _, ok := n.Channels[channelName]; !ok {
		return returnVal
	}

	socketIds := n.Channels[channelName]

	for _, socketId := range socketIds {
		if _, exists := n.Sockets[socketId]; exists {
			returnVal[socketId] = n.Sockets[socketId]
		}
	}
	return returnVal
}

// GetChannelMembers returns member data for presence channels, mapping user IDs to their member information.
// This method is specifically for presence channels and extracts user information from
// the presence data stored in each socket. It returns a map of user IDs to their
// member data, which includes user info and other presence-related information.
func (n *Namespace) GetChannelMembers(channelName constants.ChannelName) map[string]*pusherClient.MemberData {
	socketInfo := n.GetChannelSockets(channelName)
	log.Logger().Tracef("GetChannelMembers() -> socketInfo length: %d", len(socketInfo))

	members := make(map[string]*pusherClient.MemberData)

	for _, socket := range socketInfo {
		if socket.PresenceData != nil {
			if val := socket.getPresenceDataForChannel(channelName); val != nil {
				log.Logger().Tracef("GetChannelMembers() -> socket: %s, Channel: %s", socket.ID, channelName)
				members[val.UserID] = val
			}
		}
	}
	return members
}

// IsInChannel checks whether a specific WebSocket is subscribed to a given channel.
// This method provides a quick way to verify if a socket is currently subscribed
// to a channel without needing to retrieve all channel sockets.
func (n *Namespace) IsInChannel(wsID constants.SocketID, channelName constants.ChannelName) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if _, ok := n.Channels[channelName]; !ok {
		return false
	}

	if slices.Contains(n.Channels[channelName], wsID) {
		return true
	}
	return false
}

// TerminateUserConnections forcibly closes all WebSocket connections for a specific user.
// This method is used for administrative purposes to disconnect a user from all
// their active connections. It sends an error message to each connection before
// closing it, providing the user with feedback about why they were disconnected.
func (n *Namespace) TerminateUserConnections(userID string) {
	sockets := n.GetSockets()

	for _, ws := range sockets {
		if ws.userID == userID {
			errorData := &payloads.ChannelErrorData{
				Code:    int(util.ErrCodeUnauthorizedConnection),
				Message: "You have been disconnected by the app",
			}
			evt := &payloads.ChannelEvent{
				Event: constants.PusherError,
				Data:  errorData,
			}
			_d := evt.ToJson(true)
			ws.Send(_d)
			time.Sleep(100 * time.Millisecond)
			ws.closeConnection(util.ErrCodeUnauthorizedConnection)
		}
	}
}

// AddUser associates a WebSocket with a user ID for user-specific features.
// This method enables user-specific functionality like presence channels and
// user-targeted messaging. It maintains a mapping of user IDs to their associated
// socket IDs, allowing multiple sockets per user (e.g., multiple browser tabs).
func (n *Namespace) AddUser(ws *WebSocket) error {
	if ws.userID == "" {
		return nil
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Users[ws.userID]; !ok {
		n.Users[ws.userID] = []constants.SocketID{}
	}

	if !slices.Contains(n.Users[ws.userID], ws.ID) {
		log.Logger().Tracef("Adding %s to %s userID", ws.ID, ws.userID)
		n.Users[ws.userID] = append(n.Users[ws.userID], ws.ID)
	}
	return nil
}

// RemoveUser disassociates a WebSocket from its user ID.
// This method is called when a socket is disconnected or when a user signs out.
// It removes the socket ID from the user's socket list and cleans up the user
// entry if no more sockets are associated with that user.
func (n *Namespace) RemoveUser(ws *WebSocket) error {
	if ws.userID == "" {
		return nil
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Users[ws.userID]; ok {
		// Create a new slice without the websocketId to avoid keeping references to the original array
		originalSockets := n.Users[ws.userID]
		newSockets := make([]constants.SocketID, 0, len(originalSockets)-1)

		for _, id := range originalSockets {
			if id != ws.ID {
				newSockets = append(newSockets, id)
			}
		}

		// Replace the user's slice with the new one
		n.Users[ws.userID] = newSockets

		if len(n.Users[ws.userID]) == 0 {
			delete(n.Users, ws.userID)
		}
	}
	return nil
}

// GetUserSockets returns all WebSocket connections associated with a specific user ID.
// This method is useful for user-specific operations like sending messages to all
// of a user's active connections or checking if a user is online. It filters out
// any sockets that may have been disconnected but not yet cleaned up.
func (n *Namespace) GetUserSockets(userID string) []*WebSocket {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Users[userID]; !ok {
		return nil
	}
	websocketIDs := n.Users[userID]

	if len(websocketIDs) == 0 {
		return make([]*WebSocket, 0)
	}

	sockets := make([]*WebSocket, 0)
	for _, socketID := range websocketIDs {
		if _, ok := n.Sockets[socketID]; ok {
			sockets = append(sockets, n.Sockets[socketID])
			continue
		}
	}
	return sockets
}
