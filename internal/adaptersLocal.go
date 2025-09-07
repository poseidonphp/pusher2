package internal

import (
	"errors"
	"strings"
	"sync"
	"time"

	"pusher/internal/constants"
	"pusher/internal/metrics"
	"pusher/internal/util"
	"pusher/log"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

type LocalAdapter struct {
	Namespaces     map[constants.AppID]*Namespace
	mutex          sync.Mutex
	metricsManager metrics.MetricsInterface
}

// NewLocalAdapter creates a new LocalAdapter with metrics support
func NewLocalAdapter(metricsManager metrics.MetricsInterface) *LocalAdapter {
	return &LocalAdapter{
		Namespaces:     make(map[constants.AppID]*Namespace),
		metricsManager: metricsManager,
	}
}

func (l *LocalAdapter) createBlankNamespace() *Namespace {
	n := &Namespace{
		Channels: make(map[constants.ChannelName][]constants.SocketID),
		Sockets:  make(map[constants.SocketID]*WebSocket),
		Users:    make(map[string][]constants.SocketID),
	}

	// start a go routine to call n.CompactMaps() every 5 minutes
	go func() {
		for {
			// wait for 5 minutes
			<-time.After(5 * time.Minute)
			n.CompactMaps()
		}
	}()

	return n
}

// updateChannelMetrics updates channel-related metrics for the LocalAdapter
func (l *LocalAdapter) updateChannelMetrics(appID constants.AppID) {

	// Get all channels and categorize them
	// All of the calls to this method are made with the mutex locked, so we need to unlock it before calling GetChannelsWithSocketsCount to avoid a deadlock
	// and then lock it again afterwards
	l.mutex.Unlock()
	channels := l.GetChannelsWithSocketsCount(appID, false)
	l.mutex.Lock()
	//
	// Calculate channel counts by type
	totalChannelsCount := int64(len(channels))
	presenceChannelsCount := int64(0)
	privateChannelsCount := int64(0)
	publicChannelsCount := int64(0)

	for channelName := range channels {
		if util.IsPresenceChannel(channelName) {
			presenceChannelsCount++
		} else if util.IsPrivateChannel(channelName) {
			privateChannelsCount++
		} else {
			publicChannelsCount++
		}
	}

	// Update metrics
	l.metricsManager.SetChannelsTotal(float64(totalChannelsCount))
	l.metricsManager.SetPresenceChannels(float64(presenceChannelsCount))
	l.metricsManager.SetPrivateChannels(float64(privateChannelsCount))
	l.metricsManager.SetPublicChannels(float64(publicChannelsCount))
}

func (l *LocalAdapter) Init() error {
	l.Namespaces = make(map[constants.AppID]*Namespace)
	return nil
}

func (l *LocalAdapter) GetNamespace(appID constants.AppID) (*Namespace, error) {
	if ns, ok := l.Namespaces[appID]; ok {
		return ns, nil
	}
	return nil, errors.New("namespace not found")
}

func (l *LocalAdapter) GetNamespaces() (map[constants.AppID]*Namespace, error) {
	return l.Namespaces, nil
}

func (l *LocalAdapter) AddSocket(appID constants.AppID, ws *WebSocket) error {
	if l.Namespaces == nil {
		return errors.New("adapter not initialized")
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		// log.Logger().Warn("namespace not found, creating new one")
		l.Namespaces[appID] = l.createBlankNamespace()
	}

	if !l.Namespaces[appID].AddSocket(ws) {
		return errors.New("socket already exists")
	}

	// Update channel metrics after adding socket
	l.updateChannelMetrics(appID)
	return nil
}

func (l *LocalAdapter) RemoveSocket(appID constants.AppID, wsID constants.SocketID) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; ok {
		l.Namespaces[appID].RemoveSocket(wsID)
		// Update channel metrics after removing socket
		l.updateChannelMetrics(appID)
	}
	return nil
}

func (l *LocalAdapter) AddToChannel(appID constants.AppID, channel constants.ChannelName, ws *WebSocket) (int64, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return 0, errors.New("namespace not found")
	}

	count := l.Namespaces[appID].AddToChannel(ws, channel)
	// Update channel metrics after adding to channel
	l.updateChannelMetrics(appID)
	return count, nil
}

func (l *LocalAdapter) RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return 0
	}
	count := l.Namespaces[appID].RemoveFromChannel(wsID, channels)
	// Update channel metrics after removing from channel
	l.updateChannelMetrics(appID)
	return count
}

func (l *LocalAdapter) Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptIDs ...constants.SocketID) error {
	// For user-dedicated channels, intercept the call and use custom logic
	if strings.HasPrefix(string(channel), constants.SocketRushServerToUserPrefix) {
		userId := strings.TrimPrefix(string(channel), constants.SocketRushServerToUserPrefix)
		sockets, err := l.GetUserSockets(appID, userId)
		if err != nil {
			return err
		}
		for _, socket := range sockets {
			socket.Send(data)
		}
		return nil
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return errors.New("namespace not found")
	}

	sockets := l.Namespaces[appID].GetChannelSockets(channel)

	// see if we have any socket id's to exclude; there should only ever be one despite the fact that we accept a slice which we do so it can be an optional field
	hasExceptingIds := len(exceptIDs) > 0 && exceptIDs[0] != ""

	for socketId, socket := range sockets {
		if hasExceptingIds && socketId == exceptIDs[0] {
			continue
		}
		socket.Send(data)
	}
	return nil
}

func (l *LocalAdapter) TerminateUserConnections(appID constants.AppID, userID string) {
	// l.mutex.Lock()
	if _, ok := l.Namespaces[appID]; ok {
		// l.mutex.Unlock()
		l.Namespaces[appID].TerminateUserConnections(userID)
	} else {
		// l.mutex.Unlock()
	}
	log.Logger().Tracef("Terminated connections for user %s in app %s", userID, appID)
}

func (l *LocalAdapter) Disconnect() {
	// not used for the local adapter
	return
}

func (l *LocalAdapter) ClearNamespace(appID constants.AppID) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; ok {
		l.Namespaces[appID] = l.createBlankNamespace()
	}
}

func (l *LocalAdapter) ClearNamespaces() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.Namespaces = make(map[constants.AppID]*Namespace)
}

func (l *LocalAdapter) GetSockets(appID constants.AppID, _ bool) map[constants.SocketID]*WebSocket {
	l.mutex.Lock()
	if _, ok := l.Namespaces[appID]; !ok {
		l.mutex.Unlock()
		return make(map[constants.SocketID]*WebSocket)
	}
	l.mutex.Unlock()

	return l.Namespaces[appID].GetSockets()
}

func (l *LocalAdapter) GetSocketsCount(appID constants.AppID, _ bool) int64 {
	l.mutex.Lock()
	if _, ok := l.Namespaces[appID]; !ok {
		l.mutex.Unlock()
		return 0
	}
	l.mutex.Unlock()

	sockets := l.Namespaces[appID].GetSockets()
	return int64(len(sockets))
}

func (l *LocalAdapter) GetChannels(appID constants.AppID, _ bool) map[constants.ChannelName][]constants.SocketID {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return nil
	}
	channels := l.Namespaces[appID].GetChannels()
	return channels
}

func (l *LocalAdapter) GetChannelsWithSocketsCount(appID constants.AppID, _ bool) map[constants.ChannelName]int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return make(map[constants.ChannelName]int64)
	}
	channels := l.Namespaces[appID].GetChannelsWithSocketsCount()
	return channels
}

func (l *LocalAdapter) GetChannelSockets(appID constants.AppID, channel constants.ChannelName, _ bool) map[constants.SocketID]*WebSocket {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return make(map[constants.SocketID]*WebSocket)
	}
	return l.Namespaces[appID].GetChannelSockets(channel)

}

func (l *LocalAdapter) GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, _ bool) int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return int64(0)
	}
	sockets := l.Namespaces[appID].GetChannelSockets(channel)
	return int64(len(sockets))
}

func (l *LocalAdapter) GetChannelMembers(appID constants.AppID, channel constants.ChannelName, _ bool) map[string]*pusherClient.MemberData {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return make(map[string]*pusherClient.MemberData)
	}
	members := l.Namespaces[appID].GetChannelMembers(channel)
	return members
}

func (l *LocalAdapter) GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, _ bool) int {
	l.mutex.Lock()

	if _, ok := l.Namespaces[appID]; !ok {
		l.mutex.Unlock()
		return 0
	}
	l.mutex.Unlock()
	members := l.Namespaces[appID].GetChannelMembers(channel)
	return len(members)
}

func (l *LocalAdapter) IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, _ bool) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return false
	}
	return l.Namespaces[appID].IsInChannel(wsID, channel)
}

func (l *LocalAdapter) AddUser(appID constants.AppID, ws *WebSocket) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return errors.New("namespace not found")
	}
	return l.Namespaces[appID].AddUser(ws)
}

func (l *LocalAdapter) RemoveUser(appID constants.AppID, ws *WebSocket) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return errors.New("namespace not found")
	}
	return l.Namespaces[appID].RemoveUser(ws)
}

func (l *LocalAdapter) GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return nil, errors.New("namespace not found")
	}
	sockets := l.Namespaces[appID].GetUserSockets(userID)
	return sockets, nil
}

func (l *LocalAdapter) GetPresenceChannelsWithUsersCount(appID constants.AppID, _ bool) map[constants.ChannelName]int64 {
	log.Logger().Tracef("called GetPresenceChannelsWithUsersCount() within localAdapter")
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.Namespaces[appID]; !ok {
		return make(map[constants.ChannelName]int64)
	}
	presenceChannels := l.Namespaces[appID].GetPresenceChannelsWithUsersCount()
	return presenceChannels
}
