package internal

import (
	"pusher/internal/constants"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

// AdapterInterface defines the contract for managing WebSocket connections, channels, and messaging
// across different storage backends (local memory, Redis, etc.). It provides a unified interface
// for handling real-time communication features like pub/sub, presence channels, and user management.
type AdapterInterface interface {
	// ===== LIFECYCLE MANAGEMENT =====

	// Init initializes the adapter with any required setup (connections, data structures, etc.)
	Init() error

	// Disconnect gracefully shuts down the adapter and closes any external connections
	Disconnect()

	// ===== NAMESPACE MANAGEMENT =====

	// GetNamespace retrieves a specific namespace (app) by its ID, containing all channels and sockets for that app
	GetNamespace(appID constants.AppID) (*Namespace, error)

	// GetNamespaces returns all namespaces managed by this adapter
	GetNamespaces() (map[constants.AppID]*Namespace, error)

	// ClearNamespace removes all data (sockets, channels, users) for a specific app
	ClearNamespace(appID constants.AppID)

	// ClearNamespaces removes all data for all apps managed by this adapter
	ClearNamespaces()

	// ===== SOCKET MANAGEMENT =====

	// AddSocket adds a new WebSocket connection to the specified app's namespace
	AddSocket(appID constants.AppID, ws *WebSocket) error

	// RemoveSocket removes a WebSocket connection from the specified app's namespace
	RemoveSocket(appID constants.AppID, wsID constants.SocketID) error

	// GetSockets returns all WebSocket connections for an app. The onlyLocal parameter determines
	// whether to return only local connections (true) or aggregate across all nodes (false).
	GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket

	// GetSocketsCount returns the total number of WebSocket connections for an app
	GetSocketsCount(appID constants.AppID, onlyLocal bool) int64

	// ===== CHANNEL MANAGEMENT =====

	// AddToChannel subscribes a WebSocket to a specific channel within an app's namespace.
	// Returns the number of connections in the channel after adding this socket.
	AddToChannel(appID constants.AppID, channel constants.ChannelName, ws *WebSocket) (int64, error)

	// RemoveFromChannel unsubscribes a WebSocket from one or more channels.
	// Returns the number of remaining connections in the channel (0 if multiple channels specified).
	RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64

	// GetChannels returns all channels and their subscribed socket IDs for an app
	GetChannels(appID constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID

	// GetChannelsWithSocketsCount returns a map of channel names to their connection counts
	GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64

	// GetChannelSockets returns all WebSocket connections subscribed to a specific channel
	GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket

	// GetChannelSocketsCount returns the number of WebSocket connections subscribed to a specific channel
	GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int64

	// IsInChannel checks whether a specific WebSocket is subscribed to a given channel
	IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool

	// ===== MESSAGING =====

	// Send broadcasts a message to all sockets subscribed to a channel within an app's namespace.
	// The exceptingIds parameter allows excluding specific socket IDs from receiving the message.
	Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error

	// ===== PRESENCE CHANNEL MANAGEMENT =====

	// GetChannelMembers returns member data for presence channels, mapping user IDs to their member information
	GetChannelMembers(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[string]*pusherClient.MemberData

	// GetChannelMembersCount returns the number of unique users in a presence channel
	GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int

	// GetPresenceChannelsWithUsersCount returns a map of presence channel names to their user counts
	GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64

	// ===== USER MANAGEMENT =====

	// AddUser associates a WebSocket with a user ID for user-specific features and presence channels
	AddUser(appID constants.AppID, ws *WebSocket) error

	// RemoveUser disassociates a WebSocket from its user ID
	RemoveUser(appID constants.AppID, ws *WebSocket) error

	// GetUserSockets returns all WebSocket connections associated with a specific user ID
	GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error)

	// TerminateUserConnections forcibly closes all WebSocket connections for a specific user across all channels
	TerminateUserConnections(appID constants.AppID, userID string)
}
