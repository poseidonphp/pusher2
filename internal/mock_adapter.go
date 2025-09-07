package internal

import (
	"errors"

	"pusher/internal/constants"

	pusherClient "github.com/pusher/pusher-http-go/v5"
)

// MockAdapter is a base mock adapter that implements AdapterInterface
type MockAdapter struct {
	namespaces map[constants.AppID]*Namespace
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		namespaces: make(map[constants.AppID]*Namespace),
	}
}

func (m *MockAdapter) Init() error { return nil }
func (m *MockAdapter) Disconnect() {}
func (m *MockAdapter) GetNamespace(appID constants.AppID) (*Namespace, error) {
	if ns, exists := m.namespaces[appID]; exists {
		return ns, nil
	}
	return nil, errors.New("namespace not found")
}
func (m *MockAdapter) GetNamespaces() (map[constants.AppID]*Namespace, error) {
	return m.namespaces, nil
}
func (m *MockAdapter) ClearNamespace(appID constants.AppID) {
	delete(m.namespaces, appID)
}
func (m *MockAdapter) ClearNamespaces() {
	m.namespaces = make(map[constants.AppID]*Namespace)
}
func (m *MockAdapter) AddSocket(appID constants.AppID, ws *WebSocket) error              { return nil }
func (m *MockAdapter) RemoveSocket(appID constants.AppID, wsID constants.SocketID) error { return nil }
func (m *MockAdapter) GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return map[constants.SocketID]*WebSocket{}
}
func (m *MockAdapter) GetSocketsCount(appID constants.AppID, onlyLocal bool) int64 { return 0 }
func (m *MockAdapter) AddToChannel(appID constants.AppID, channel constants.ChannelName, ws *WebSocket) (int64, error) {
	return 1, nil
}
func (m *MockAdapter) RemoveFromChannel(appID constants.AppID, channels []constants.ChannelName, wsID constants.SocketID) int64 {
	return 0
}
func (m *MockAdapter) GetChannels(appID constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID {
	return map[constants.ChannelName][]constants.SocketID{}
}
func (m *MockAdapter) GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return map[constants.ChannelName]int64{}
}
func (m *MockAdapter) GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket {
	return map[constants.SocketID]*WebSocket{}
}
func (m *MockAdapter) GetChannelSocketsCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int64 {
	return 0
}
func (m *MockAdapter) IsInChannel(appID constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool {
	return false
}
func (m *MockAdapter) Send(appID constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error {
	return nil
}
func (m *MockAdapter) GetChannelMembers(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[string]*pusherClient.MemberData {
	return map[string]*pusherClient.MemberData{}
}
func (m *MockAdapter) GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int {
	return 0
}
func (m *MockAdapter) GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	return map[constants.ChannelName]int64{}
}
func (m *MockAdapter) AddUser(appID constants.AppID, ws *WebSocket) error    { return nil }
func (m *MockAdapter) RemoveUser(appID constants.AppID, ws *WebSocket) error { return nil }
func (m *MockAdapter) GetUserSockets(appID constants.AppID, userID string) ([]*WebSocket, error) {
	return []*WebSocket{}, nil
}
func (m *MockAdapter) TerminateUserConnections(appID constants.AppID, userID string) {}
