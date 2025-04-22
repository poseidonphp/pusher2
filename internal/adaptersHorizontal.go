package internal

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/constants"
	"pusher/log"
)

type HorizontalInterface interface {
	GetChannelName() string
	broadcastToChannel(channel string, data string)
	getNumSub() (int64, error)
}

func NewHorizontalAdapter(adapter HorizontalInterface) (*HorizontalAdapter, error) {
	ha := &HorizontalAdapter{
		concreteAdapter: adapter,
		requestTimeout:  5,
		requestChannel:  "",
		responseChannel: "",
		requests:        make(map[string]*HorizontalRequest),
		channel:         "horizontal-adapter",
		uuid:            uuid.NewString(),
	}
	err := ha.Init() // calls the init on the local adapter
	if err != nil {
		log.Logger().Errorf("Error initializing HorizontalAdapter: %v", err)
		return nil, err
	}
	return ha, nil
}

type HorizontalRequestType int

const (
	SOCKETS HorizontalRequestType = iota
	CHANNELS
	CHANNEL_SOCKETS
	CHANNEL_MEMBERS
	SOCKETS_COUNT
	CHANNEL_MEMBERS_COUNT
	CHANNEL_SOCKETS_COUNT
	SOCKET_EXISTS_IN_CHANNEL
	CHANNELS_WITH_SOCKETS_COUNT
	TERMINATE_USER_CONNECTIONS
	PRESENCE_CHANNELS_WITH_USERS_COUNT
)

type HorizontalRequestExtra struct {
	numSub                   int64
	msgCount                 int64
	sockets                  map[constants.SocketID]*WebSocket
	members                  map[string]*pusherClient.MemberData
	channels                 map[string][]constants.ChannelName
	channelsWithSocketsCount map[string]int64
	totalCount               int64
}

type HorizontalRequest struct {
	requestID           string
	appID               constants.AppID
	requestType         HorizontalRequestType
	timestamp           time.Time
	requestResolveChan  chan *HorizontalResponse // used to receive incoming responses from other nodes
	requestResponseChan chan *HorizontalResponse // used to send the final aggregated responses to the requestor
	// resolve     func()
	// reject      func()
	timeout any
	HorizontalRequestExtra
}

type HorizontalRequestOptions struct {
	Opts map[string]string
}

type HorizontalRequestBody struct {
	Type      HorizontalRequestType
	RequestID string
	AppID     constants.AppID
	HorizontalRequestOptions
}

func (hrb *HorizontalRequestBody) ToJson() []byte {
	_data, err := json.Marshal(hrb)
	if err != nil {
		log.Logger().Errorf("Error marshalling HorizontalRequestBody: %v", err)
		return []byte(`{"error": "marshalling error"}"`)
	}
	return _data
}

type HorizontalResponse struct {
	RequestID                string
	Sockets                  map[constants.SocketID]*WebSocket
	Members                  map[string]*pusherClient.MemberData
	Channels                 map[constants.ChannelName][]constants.SocketID
	ChannelsWithSocketsCount map[constants.ChannelName]int64
	TotalCount               int64
	Exists                   bool
}

type horizontalPubsubBroadcastedMessage struct {
	Uuid         string
	AppID        constants.AppID
	Channel      constants.ChannelName
	Data         any
	ExceptingIds constants.SocketID
}

// HorizontalAdapter behaves similar to an abstract class; it has methods that can be used by other structs that implment
// and will satisfy the HorizontalInterface. It also has a concreteAdapter that should store an instance of the
// actual horizontal driver being used (ie redis). You should never directly assign the HorizontalAdapter as the
// adapter being used.
type HorizontalAdapter struct {
	LocalAdapter
	requestTimeout  int
	requestChannel  string
	responseChannel string
	requests        map[string]*HorizontalRequest
	channel         string
	uuid            string
	// resolvers       map[HorizontalRequestType]horizontalResolver
	concreteAdapter HorizontalInterface
}

type AdapterMessageToSend struct {
	Uuid        string
	AppID       constants.AppID
	Channel     constants.ChannelName
	Data        any
	ExceptingId constants.SocketID
}

func (ha *HorizontalAdapter) Send(appId constants.AppID, channel constants.ChannelName, data []byte, exceptingIds ...constants.SocketID) error {
	exceptId := ""
	if len(exceptingIds) > 0 {
		exceptId = exceptingIds[0]
	}
	_data, _ := json.Marshal(&AdapterMessageToSend{
		Uuid:        ha.uuid,
		AppID:       appId,
		Channel:     channel,
		Data:        string(data),
		ExceptingId: exceptId,
	})

	ha.concreteAdapter.broadcastToChannel(ha.concreteAdapter.GetChannelName(), string(_data))
	ha.sendLocally(appId, channel, string(data), exceptingIds...)
	return nil
}

func (ha *HorizontalAdapter) sendLocally(appId constants.AppID, channel constants.ChannelName, data string, exceptingIds ...constants.SocketID) {
	err := ha.LocalAdapter.Send(appId, channel, []byte(data), exceptingIds...)
	if err != nil {
		log.Logger().Errorf("Error sending locally: %v", err)
	}
}

func (ha *HorizontalAdapter) TerminateUserConnections(appId constants.AppID, userId string) {
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return
	}
	requestExtra := &HorizontalRequestExtra{
		numSub: numSub,
	}
	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"userId": userId,
		},
	}
	_ = ha.sendRequest(appId, TERMINATE_USER_CONNECTIONS, requestExtra, requestOptions)
	ha.terminateLocalUserConnections(appId, userId)
}

func (ha *HorizontalAdapter) terminateLocalUserConnections(appId constants.AppID, userId string) {
	ha.LocalAdapter.TerminateUserConnections(appId, userId)
}

func (ha *HorizontalAdapter) GetSockets(appID constants.AppID, onlyLocal bool) map[constants.SocketID]*WebSocket {
	localSockets := ha.LocalAdapter.GetSockets(appID, true)

	if onlyLocal {
		return localSockets
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localSockets
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localSockets
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:  numSub,
		sockets: localSockets,
	}

	response := ha.sendRequest(appID, SOCKETS, requestExtra, nil)
	return response.Sockets
}

func (ha *HorizontalAdapter) GetSocketsCount(appID constants.AppID, onlyLocal bool) int64 {
	socketCount := ha.LocalAdapter.GetSocketsCount(appID, true)

	if onlyLocal {
		return socketCount
	}
	numberOfSubscribers, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return socketCount
	}
	if numberOfSubscribers <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numberOfSubscribers)
		return socketCount
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:     numberOfSubscribers,
		totalCount: socketCount,
	}
	response := ha.sendRequest(appID, SOCKETS_COUNT, requestExtra, nil)
	return response.TotalCount
}

func (ha *HorizontalAdapter) GetChannels(appId constants.AppID, onlyLocal bool) map[constants.ChannelName][]constants.SocketID {
	localChannels := ha.LocalAdapter.GetChannels(appId, true)
	if onlyLocal {
		return localChannels
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localChannels
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localChannels
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:   numSub,
		channels: localChannels,
	}
	response := ha.sendRequest(appId, CHANNELS, requestExtra, nil)
	if response != nil {
		return response.Channels
	}
	return make(map[constants.ChannelName][]constants.SocketID)
}

func (ha *HorizontalAdapter) GetChannelsWithSocketsCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	localSocketCount := ha.LocalAdapter.GetChannelsWithSocketsCount(appID, true)

	if onlyLocal {
		return localSocketCount
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localSocketCount
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localSocketCount
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:                   numSub,
		channelsWithSocketsCount: localSocketCount,
	}
	response := ha.sendRequest(appID, CHANNELS_WITH_SOCKETS_COUNT, requestExtra, nil)
	if response != nil {
		return response.ChannelsWithSocketsCount
	}
	return make(map[constants.ChannelName]int64)
}

func (ha *HorizontalAdapter) GetChannelSockets(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[constants.SocketID]*WebSocket {
	localChannelSockets := ha.LocalAdapter.GetChannelSockets(appID, channel, true)

	if onlyLocal {
		return localChannelSockets
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		return localChannelSockets
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localChannelSockets
	}

	requestExtras := &HorizontalRequestExtra{
		numSub:  numSub,
		sockets: localChannelSockets,
	}
	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"channel": channel,
		},
	}
	response := ha.sendRequest(appID, CHANNEL_SOCKETS, requestExtras, requestOptions)
	if response != nil {
		return response.Sockets
	}

	return make(map[constants.SocketID]*WebSocket)
}

func (ha *HorizontalAdapter) GetChannelSocketsCount(appId constants.AppID, channel constants.ChannelName, onlyLocal bool) int64 {
	localChannelSocketCount := ha.LocalAdapter.GetChannelSocketsCount(appId, channel, true)
	if onlyLocal {
		return localChannelSocketCount
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localChannelSocketCount
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localChannelSocketCount
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:     numSub,
		totalCount: localChannelSocketCount,
	}

	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"channel": channel,
		},
	}

	response := ha.sendRequest(appId, CHANNEL_SOCKETS_COUNT, requestExtra, requestOptions)
	if response != nil {
		return response.TotalCount
	}
	return 0
}

func (ha *HorizontalAdapter) GetChannelMembers(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) map[string]*pusherClient.MemberData {
	localChannelMembers := ha.LocalAdapter.GetChannelMembers(appID, channel, true)

	if onlyLocal {
		return localChannelMembers
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		return localChannelMembers
	}
	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localChannelMembers
	}

	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"channel": channel,
		},
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:  numSub,
		members: localChannelMembers,
	}
	response := ha.sendRequest(appID, CHANNEL_MEMBERS, requestExtra, requestOptions)
	if response != nil {
		return response.Members
	}
	return make(map[string]*pusherClient.MemberData)
}

func (ha *HorizontalAdapter) GetChannelMembersCount(appID constants.AppID, channel constants.ChannelName, onlyLocal bool) int {
	localChannelMemberCount := ha.LocalAdapter.GetChannelMembersCount(appID, channel, true)

	if onlyLocal {
		return localChannelMemberCount
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localChannelMemberCount
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localChannelMemberCount
	}

	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"channel": channel,
		},
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:     numSub,
		totalCount: int64(localChannelMemberCount),
	}
	response := ha.sendRequest(appID, CHANNEL_MEMBERS_COUNT, requestExtra, requestOptions)
	if response != nil {
		return int(response.TotalCount)
	}
	return 0
}

func (ha *HorizontalAdapter) IsInChannel(appId constants.AppID, channel constants.ChannelName, wsID constants.SocketID, onlyLocal bool) bool {
	localIsInChannel := ha.LocalAdapter.IsInChannel(appId, channel, wsID, true)

	if onlyLocal {
		return localIsInChannel
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localIsInChannel
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localIsInChannel
	}

	requestOptions := &HorizontalRequestOptions{
		Opts: map[string]string{
			"channel":  channel,
			"socketId": wsID,
		},
	}
	requestExtra := &HorizontalRequestExtra{
		numSub: numSub,
	}
	response := ha.sendRequest(appId, SOCKET_EXISTS_IN_CHANNEL, requestExtra, requestOptions)
	if response != nil {
		return response.Exists
	}

	return false
}

func (ha *HorizontalAdapter) GetPresenceChannelsWithUsersCount(appID constants.AppID, onlyLocal bool) map[constants.ChannelName]int64 {
	log.Logger().Tracef("called GetPresenceChannelsWithUsersCount() within horizontalAdapter")
	localPresenceChannelsWithUsersCount := ha.LocalAdapter.GetPresenceChannelsWithUsersCount(appID, true)

	if onlyLocal {
		return localPresenceChannelsWithUsersCount
	}
	numSub, err := ha.concreteAdapter.getNumSub()
	if err != nil {
		log.Logger().Errorf("Error getting number of subscribers: %v", err)
		return localPresenceChannelsWithUsersCount
	}

	if numSub <= 1 {
		log.Logger().Tracef("getNumSub() -> num: %d", numSub)
		return localPresenceChannelsWithUsersCount
	}

	requestExtra := &HorizontalRequestExtra{
		numSub:                   numSub,
		channelsWithSocketsCount: localPresenceChannelsWithUsersCount,
	}
	response := ha.sendRequest(appID, PRESENCE_CHANNELS_WITH_USERS_COUNT, requestExtra, nil)
	if response != nil {
		return response.ChannelsWithSocketsCount
	}
	return make(map[constants.ChannelName]int64)
}

// onRequest listens for requests coming from other nodes
func (ha *HorizontalAdapter) onRequest(channel string, msg string) (dataToSend []byte, err error) {
	log.Logger().Tracef("Received onRequest(%s): %s", channel, msg)
	var request HorizontalRequestBody
	// Parse the request message
	err = json.Unmarshal([]byte(msg), &request)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling request: %v", err)
		return
	}

	ha.mutex.Lock()
	if _, hasRequest := ha.requests[request.RequestID]; hasRequest {
		ha.mutex.Unlock()
		// do not process requests for the same node that created the request
		log.Logger().Tracef("not processing request %s", request.RequestID)
		return
	}
	ha.mutex.Unlock()

	appId := request.AppID
	log.Logger().Debugf("onRequest() -> appId: %s, msg: %v", appId, request)

	switch request.Type {
	case SOCKETS:
		sockets := ha.GetSockets(appId, true)
		if err != nil {
			log.Logger().Errorf("Error getting sockets: %v", err)
			return nil, err
		}
		// var localSockets []*WebSocket
		// for _, socket := range sockets {
		// 	localSockets = append(localSockets, socket)
		// }
		_data2 := &HorizontalResponse{
			RequestID: request.RequestID,
			Sockets:   sockets,
		}
		dataToSend, _ = json.Marshal(_data2)
		return dataToSend, nil

	case CHANNEL_SOCKETS:
		if ch, exists := request.HorizontalRequestOptions.Opts["channel"]; exists {
			localSockets := ha.GetChannelSockets(appId, ch, true)
			if err != nil {
				log.Logger().Errorf("Error getting channel sockets: %v", err)
				return nil, err
			}
			_data2 := &HorizontalResponse{
				RequestID: request.RequestID,
				Sockets:   localSockets,
			}
			dataToSend, _ = json.Marshal(_data2)
			return dataToSend, nil
		}
		break
	case CHANNELS:
		localChannels := ha.GetChannels(appId, true)
		// var channels []constants.ChannelName
		var channels map[constants.ChannelName][]constants.SocketID
		for ch, sockets := range localChannels {
			channels[ch] = sockets
			// channels = append(channels, ch)
		}

		_data2 := &HorizontalResponse{
			RequestID: request.RequestID,
			Channels:  channels,
		}
		dataToSend, _ = json.Marshal(_data2)
		return dataToSend, nil
	case CHANNELS_WITH_SOCKETS_COUNT:
		localCounts := ha.GetChannelsWithSocketsCount(appId, true)
		if err != nil {
			log.Logger().Errorf("Error getting channels with sockets count: %v", err)
			return nil, err
		}

		_data2 := &HorizontalResponse{
			RequestID:                request.RequestID,
			ChannelsWithSocketsCount: localCounts,
		}
		dataToSend, _ = json.Marshal(_data2)
		return dataToSend, nil
	// CHANNEL_MEMBERS returns ChannelMembersRequestResponse
	case CHANNEL_MEMBERS:
		if ch, exists := request.HorizontalRequestOptions.Opts["channel"]; exists {
			localMembers := ha.GetChannelMembers(appId, constants.ChannelName(ch), true)
			if err != nil {
				log.Logger().Errorf("Error getting channel members: %v", err)
				return nil, err
			}
			_data2 := &HorizontalResponse{
				RequestID: request.RequestID,
				Members:   localMembers,
			}
			dataToSend, _ = json.Marshal(_data2)
			return dataToSend, nil
		}
		break
	// SOCKETS_COUNT returns SocketCountRequestResponse
	case SOCKETS_COUNT:
		localCount := ha.GetSocketsCount(appId, true)
		if err != nil {
			log.Logger().Errorf("Error getting sockets count: %v", err)
			return nil, err
		}
		_data2 := &HorizontalResponse{
			RequestID:  request.RequestID,
			TotalCount: localCount,
		}
		dataToSend, _ = json.Marshal(_data2)
		return dataToSend, nil
	// CHANNEL_MEMBERS_COUNT returns SocketCountRequestResponse
	case CHANNEL_MEMBERS_COUNT: // returns SocketCountRequestResponse
		if ch, exists := request.HorizontalRequestOptions.Opts["channel"]; exists {
			localCount := ha.GetChannelMembersCount(appId, ch, true)
			if err != nil {
				log.Logger().Errorf("Error getting channel members count: %v", err)
				return nil, err
			}
			_data2 := &HorizontalResponse{
				RequestID:  request.RequestID,
				TotalCount: int64(localCount),
			}
			dataToSend, _ = json.Marshal(_data2)
			return dataToSend, nil
		}
		break
	case CHANNEL_SOCKETS_COUNT: // returns SocketCountRequestResponse
		if ch, exists := request.HorizontalRequestOptions.Opts["channel"]; exists {
			localCount := ha.GetChannelSocketsCount(appId, ch, true)
			_data2 := &HorizontalResponse{
				RequestID:  request.RequestID,
				TotalCount: localCount,
			}
			dataToSend, _ = json.Marshal(_data2)
			return dataToSend, nil
		}
		break
	// SOCKET_EXISTS_IN_CHANNEL returns SocketExistsInChannelRequestResponse
	case SOCKET_EXISTS_IN_CHANNEL:
		if ch, channelExists := request.HorizontalRequestOptions.Opts["channel"]; channelExists {
			if wsId, socketExists := request.HorizontalRequestOptions.Opts["socketId"]; socketExists {
				existsInChannel := ha.IsInChannel(appId, ch, wsId, true)
				_data2 := &HorizontalResponse{
					RequestID: request.RequestID,
					Exists:    existsInChannel,
				}
				dataToSend, _ = json.Marshal(_data2)
				return dataToSend, nil
			}
		}
		break
	case TERMINATE_USER_CONNECTIONS:
		if uid, userIdExists := request.HorizontalRequestOptions.Opts["userId"]; userIdExists {
			ha.TerminateUserConnections(appId, uid)
			dataToSend = []byte(`{"success": true}`)
			return dataToSend, nil
		}
		break
	case PRESENCE_CHANNELS_WITH_USERS_COUNT:
		localCounts := ha.GetPresenceChannelsWithUsersCount(appId, true)
		log.Logger().Infof("localCounts: %v", localCounts)
		if err != nil {
			log.Logger().Errorf("Error getting presence channels with users count: %v", err)
			return nil, err
		}
		_data2 := &HorizontalResponse{
			RequestID:                request.RequestID,
			ChannelsWithSocketsCount: localCounts,
		}
		dataToSend, _ = json.Marshal(_data2)
		return dataToSend, nil

	default:
		log.Logger().Errorf("Unknown type: %v", request.Type)

	}
	return nil, nil
}

// onResponse handles a response from another node
func (ha *HorizontalAdapter) onResponse(channel string, msg string) {
	log.Logger().Tracef("Called onResponse() -> channel: %s, msg: %s", channel, msg)
	var response HorizontalResponse
	err := json.Unmarshal([]byte(msg), &response)
	if err != nil {
		log.Logger().Errorf("Error unmarshalling response: %v", err)
		return
	}

	ha.mutex.Lock()
	if request, hasRequest := ha.requests[response.RequestID]; hasRequest {
		ha.mutex.Unlock()
		// process the response
		log.Logger().Trace("Processing response for request ID: ", response.RequestID)

		if request.requestResolveChan != nil {
			request.requestResolveChan <- &response
		} else {
			log.Logger().Errorf("Tried to send response to nil channel request.requestResolveChan")
		}
		// ha.processReceiveResponse(response, request, false)
	} else {
		ha.mutex.Unlock()
	}
}

// sendRequest sends a request to other nodes to get data from them
func (ha *HorizontalAdapter) sendRequest(appId constants.AppID, requestType HorizontalRequestType, requestExtra *HorizontalRequestExtra, requestOptions *HorizontalRequestOptions) *HorizontalResponse {
	requestID := uuid.NewString()
	newRequest := &HorizontalRequest{
		requestID:           requestID,
		appID:               appId,
		requestType:         requestType,
		timestamp:           time.Now(),
		requestResolveChan:  make(chan *HorizontalResponse),
		requestResponseChan: make(chan *HorizontalResponse),
	}
	newRequest.msgCount = 1
	if requestExtra != nil {
		log.Logger().Errorf("Got request extra data: %v", requestExtra)
		newRequest.numSub = requestExtra.numSub
		newRequest.msgCount = requestExtra.msgCount
		newRequest.sockets = requestExtra.sockets
		newRequest.members = requestExtra.members
		newRequest.channels = requestExtra.channels
		newRequest.channelsWithSocketsCount = requestExtra.channelsWithSocketsCount
		newRequest.totalCount = requestExtra.totalCount
	}
	if newRequest.sockets == nil {
		newRequest.sockets = make(map[constants.SocketID]*WebSocket)
	}
	if newRequest.members == nil {
		newRequest.members = make(map[string]*pusherClient.MemberData)
	}
	if newRequest.channels == nil {
		newRequest.channels = make(map[constants.ChannelName][]constants.SocketID)
	}
	if newRequest.channelsWithSocketsCount == nil {
		newRequest.channelsWithSocketsCount = make(map[constants.ChannelName]int64)
	}

	ha.mutex.Lock()
	ha.requests[requestID] = newRequest
	ha.mutex.Unlock()

	// start listener for response
	go ha.waitForResponse(newRequest)

	// send request to channel by calling concrete.broadcastToChannel()
	body := &HorizontalRequestBody{
		Type:      requestType,
		RequestID: requestID,
		AppID:     appId,
	}
	if requestOptions != nil {
		body.Opts = requestOptions.Opts
	}
	ha.concreteAdapter.broadcastToChannel(ha.requestChannel, string(body.ToJson()))
	// todo metrics markHorizontalAdapterRequestSent

	// monitor the requestResolveChan and watch for an incoming message HorizontalResponse
	// if the message is not received within the timeout, return an error
	// if the message is received, close the channel and return the response
	select {
	case response := <-newRequest.requestResponseChan:
		log.Logger().Tracef("Received final response for request ID: %s", requestID)
		close(newRequest.requestResponseChan)
		newRequest.requestResponseChan = nil
		ha.mutex.Lock()
		delete(ha.requests, requestID)
		ha.mutex.Unlock()
		return response
	}
}

func (ha *HorizontalAdapter) waitForResponse(request *HorizontalRequest) {
	if request == nil {
		return
	}
	// set a timeout, and monitor channel for incoming messages then process the response
	finalResponse := &HorizontalResponse{
		RequestID: request.requestID,
		// Sockets:                  make(map[constants.SocketID]*WebSocket),
		// Members:                  make(map[string]*pusherClient.MemberData),
		// Channels:                 make(map[constants.ChannelName][]constants.SocketID),
		// ChannelsWithSocketsCount: make(map[constants.ChannelName]int64),
		// TotalCount:               0,
		// Exists:                   false,
		Sockets:                  request.sockets,
		Members:                  request.members,
		Channels:                 request.channels,
		ChannelsWithSocketsCount: request.channelsWithSocketsCount,
		TotalCount:               request.totalCount,
		Exists:                   false,
	}

	defer func() {
		log.Logger().Warnf("Closing requestResolveChan for request ID: %s", request.requestID)
		close(request.requestResolveChan)
		request.requestResolveChan = nil
	}()

	// after the timeout, we will return any/all data we've collected so far
	var countOfMessages int64 = 0
	for {
		select {
		case <-time.After(time.Duration(ha.requestTimeout) * time.Second):
			// timeout reached, close the channel and return the response
			log.Logger().Tracef("Timeout reached for request ID: %s", request.requestID)
			request.requestResponseChan <- finalResponse
			return
		case response := <-request.requestResolveChan:
			log.Logger().Tracef("Received requestResolveChan message for request ID: %s", request.requestID)
			countOfMessages++
			if response != nil {
				log.Logger().Warnf("Received response of request type: %d", request.requestType)
				switch request.requestType {
				case SOCKETS:
					if response.Sockets != nil {
						for _, socket := range response.Sockets {
							finalResponse.Sockets[socket.ID] = socket
						}
					}
				case CHANNEL_SOCKETS:
					if response.Sockets != nil {
						for _, socket := range response.Sockets {
							finalResponse.Sockets[socket.ID] = socket
						}
					}
				case CHANNELS:
					if response.Channels != nil {
						for ch, sockets := range response.Channels {
							if _, exists := finalResponse.Channels[ch]; !exists {
								finalResponse.Channels[ch] = make([]constants.SocketID, 0)
							}
							finalResponse.Channels[ch] = append(finalResponse.Channels[ch], sockets...)
						}
					}
				case CHANNELS_WITH_SOCKETS_COUNT:
					if response.ChannelsWithSocketsCount != nil {
						for channel, count := range response.ChannelsWithSocketsCount {
							if existingCount, exists := finalResponse.ChannelsWithSocketsCount[channel]; exists {
								finalResponse.ChannelsWithSocketsCount[channel] = existingCount + count
							} else {
								finalResponse.ChannelsWithSocketsCount[channel] = count
							}
						}
					}
				case CHANNEL_MEMBERS:
					log.Logger().Warnf("Received CHANNEL_MEMBERS response: %v", response)
					if response.Members != nil {
						for id, member := range response.Members {
							finalResponse.Members[id] = member
						}
					}
				case SOCKETS_COUNT:
					if response.TotalCount != 0 {
						finalResponse.TotalCount += response.TotalCount
					}
				case CHANNEL_MEMBERS_COUNT:
					if response.TotalCount != 0 {
						finalResponse.TotalCount += response.TotalCount
					}
				case CHANNEL_SOCKETS_COUNT:
					if response.TotalCount != 0 {
						finalResponse.TotalCount += response.TotalCount
					}
				case SOCKET_EXISTS_IN_CHANNEL:
					if response.Exists {
						finalResponse.Exists = true
						// in this case, since it exists on at least one node, we can return without waiting for other nodes to report
						request.requestResponseChan <- finalResponse
						return
					}
				// case TERMINATE_USER_CONNECTIONS:
				// 	// no need to process this response, just return
				case PRESENCE_CHANNELS_WITH_USERS_COUNT:
					log.Logger().Warnf("Received PRESENCE_CHANNELS_WITH_USERS_COUNT response: %v", response)
					if response.ChannelsWithSocketsCount != nil {
						for channel, count := range response.ChannelsWithSocketsCount {
							if existingCount, exists := finalResponse.ChannelsWithSocketsCount[channel]; exists {
								finalResponse.ChannelsWithSocketsCount[channel] = existingCount + count
							} else {
								finalResponse.ChannelsWithSocketsCount[channel] = count
							}
						}
					}
				}
			}
			// check if the number of messages received is equal to the number of subscribers
			if countOfMessages == (request.numSub - 1) {
				// all messages received, close the channel and return the response
				log.Logger().Tracef("All messages received for request ID: %s", request.requestID)
				request.requestResponseChan <- finalResponse
				return
			} else {
				log.Logger().Tracef("Still waiting for more responses: %d/%d", countOfMessages, request.numSub-1)
			}
		}
	}

}
