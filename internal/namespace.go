package internal

import (
	"slices"
	"sync"
	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

// This file will mimic the namespace.ts file from soketi

type Namespace struct {
	Channels map[constants.ChannelName][]constants.SocketID // list of Channel connections for the current app
	Sockets  map[constants.SocketID]*WebSocket              // list of sockets connected to the namespace
	Users    map[string][]constants.SocketID                // list of user id's and their associated socket ids
	mutex    sync.Mutex
}

func (n *Namespace) GetSockets() map[constants.SocketID]*WebSocket {
	//
	return n.Sockets
}

func (n *Namespace) AddSocket(ws *WebSocket) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Sockets[ws.ID]; ok {
		return false
	}
	n.Sockets[ws.ID] = ws
	return true
}

func (n *Namespace) RemoveSocket(wsID constants.SocketID) {
	n.mutex.Lock()
	var channelKeys []constants.ChannelName
	for channel := range n.Channels {
		channelKeys = append(channelKeys, channel)
	}
	n.mutex.Unlock()
	// Also ensure the user is removed if this is a user socket
	if socket, exists := n.Sockets[wsID]; exists && socket.userID != "" {
		_ = n.RemoveUser(socket)
	}

	n.mutex.Lock()
	if _, ok := n.Sockets[wsID]; ok {
		n.Sockets[wsID] = nil
		delete(n.Sockets, wsID)
	}
	n.mutex.Unlock()

	// Remove socket from all channels
	for _, channel := range channelKeys {
		n.RemoveFromChannel(wsID, []constants.ChannelName{channel})
	}

}

// func (n *Namespace) RemoveSocket(wsID constants.SocketID) {
// 	n.mutex.Lock()
// 	var channelKeys []constants.ChannelName
// 	for channel := range n.Channels {
// 		channelKeys = append(channelKeys, channel)
// 	}
// 	if _, ok := n.Sockets[wsID]; ok {
// 		delete(n.Sockets, wsID)
// 	}
// 	n.mutex.Unlock()
// 	_ = n.RemoveFromChannel(wsID, channelKeys)
// }

func (n *Namespace) AddToChannel(ws *WebSocket, channel constants.ChannelName) int64 {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Channels[channel]; !ok {
		n.Channels[channel] = []constants.SocketID{}
	}
	n.Channels[channel] = append(n.Channels[channel], ws.ID)
	return int64(len(n.Channels[channel]))
}

// RemoveFromChannel removes the socket from the Channel, and removes the Channel if it is empty.
// If multiple channels are passed, it will always return 0. Otherwise, it will return the number of connections left in the Channel.
func (n *Namespace) RemoveFromChannel(websocketId constants.SocketID, channels []constants.ChannelName) int64 {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	remove := func(channel constants.ChannelName) int64 {
		if _, ok := n.Channels[channel]; ok {
			// delete the websocketId from the Channel
			for i, id := range n.Channels[channel] {
				if id == websocketId {
					n.Channels[channel] = append(n.Channels[channel][:i], n.Channels[channel][i+1:]...)
					break
				}
			}
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

func (n *Namespace) GetChannels() map[constants.ChannelName][]constants.SocketID {
	return n.Channels
}

func (n *Namespace) GetChannelsWithSocketsCount() map[constants.ChannelName]int64 {
	channels := n.GetChannels()
	list := make(map[constants.ChannelName]int64)

	for channel, sockets := range channels {
		list[channel] = int64(len(sockets))
	}
	return list
}

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

func (n *Namespace) RemoveUser(ws *WebSocket) error {
	if ws.userID == "" {
		return nil
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, ok := n.Users[ws.userID]; ok {
		for i, id := range n.Users[ws.userID] {
			if id == ws.ID {
				n.Users[ws.userID] = slices.Delete(n.Users[ws.userID], i, i+1)
				break
			}
		}
		if len(n.Users[ws.userID]) == 0 {
			delete(n.Users, ws.userID)
		}
	}
	return nil
}

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

func (n *Namespace) GetPresenceChannelsWithUsersCount() map[constants.ChannelName]int64 {
	log.Logger().Tracef("called GetPresenceChannelsWithUsersCount() within namespace")
	// n.mutex.Lock()
	// defer n.mutex.Unlock()
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

func (n *Namespace) CompactMaps() {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Create new maps with appropriate capacity
	newChannels := make(map[constants.ChannelName][]constants.SocketID, len(n.Channels))

	// Copy only non-empty channels
	for channel, sockets := range n.Channels {
		if sockets != nil && len(sockets) > 0 {
			newChannels[channel] = sockets
		}
	}

	n.Channels = newChannels

	// Similar process for other maps
	// do the same for n.Sockets
	newSockets := make(map[constants.SocketID]*WebSocket, len(n.Sockets))
	for socketID, socket := range n.Sockets {
		if socket != nil {
			newSockets[socketID] = socket
		}
	}
	n.Sockets = newSockets
}
