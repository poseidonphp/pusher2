package internal

import (
	"encoding/json"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/storage"
	"pusher/internal/util"
	"pusher/log"
	"sync"
)

type Channel struct {
	Name        constants.ChannelName
	Connections map[constants.SocketID]bool
	Type        constants.ChannelType
}

type ChannelEvent struct {
	Event    string                `json:"event"`
	Channel  constants.ChannelName `json:"channel"`
	Data     string                `json:"data"`
	UserID   string                `json:"user_id,omitempty"`   // optional, present only if this is a `client event` on a `presence channel`
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

// ValidatePresenceChannelRequirements - Checks if the presence channel requirements are met for a given request
func ValidatePresenceChannelRequirements(channel constants.ChannelName, userData string) (presenceMemberData pusherClient.MemberData, err *util.Error) {
	if storage.Manager.GetChannelCount(channel) >= config.MaxPresenceUsers {
		err = util.NewError(util.ErrCodeMaxPresenceSubscribers)
	}

	if len(userData) > config.MaxPresenceUserDataBytes {
		err = util.NewError(util.ErrCodePresenceUserDataTooMuch)
	}

	uErr := json.Unmarshal([]byte(userData), &presenceMemberData)
	if uErr != nil {
		log.Logger().Errorf("Error unmarshalling presence channel data: %s", err)
		err = util.NewError(util.ErrCodeInvalidPayload)
	}
	if len(presenceMemberData.UserID) > constants.MaxPresenceUserIDLength {
		err = util.NewError(util.ErrCodePresenceUserIDTooLong)
	}

	return
}

func (c *Channel) addSocketID(socketID constants.SocketID) {
	mutex := sync.Mutex{}
	mutex.Lock()
	defer mutex.Unlock()
	c.Connections[socketID] = true
}

//func (c *Channel) broadcastMemberAddedEvent(memberData pusherClient.MemberData) {
//	md, e := json.Marshal(memberData)
//	if e != nil {
//		log.Logger().Errorf("Error marshalling member data: %s", e)
//		return
//	}
//	channelEvent := ChannelEvent{
//		Event:   constants.PusherInternalPresenceMemberAdded,
//		Channel: c.Name,
//		Data:    string(md),
//	}
//	err := GlobalHub.publishChannelEventLocally(channelEvent)
//	if err != nil {
//		log.Logger().Errorf("Error broadcasting member_added event: %s", err)
//	}
//}
//
//func (c *Channel) broadcastEventOnChannel(event ChannelEvent) {
//
//}
