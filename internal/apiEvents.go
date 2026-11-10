package internal

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"pusher/internal/apps"
	"pusher/internal/cache"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

// var maxTriggerableChannels = 100

// BatchEventForm ...
type BatchEventForm struct {
	Batch []*payloads.PusherApiMessage `json:"batch"`
}

func checkMessageToBroadcast(w http.ResponseWriter, app *apps.App, apiMsg *payloads.PusherApiMessage) error {
	if apiMsg.Channel != "" && (apiMsg.Channels == nil || len(apiMsg.Channels) == 0) {
		apiMsg.Channels = []constants.ChannelName{apiMsg.Channel}
	}

	// Check if the Channel is valid
	for _, channel := range apiMsg.Channels {
		if !util.ValidChannel(channel, app.MaxChannelNameLength) {
			writeError(w, http.StatusBadRequest, util.ErrInvalidChannel.Error())
			tuncatedChannelName := channel
			if len(channel) > 20 {
				tuncatedChannelName = channel[:20] + "..."
			}
			return fmt.Errorf("invalid Channel: %s", tuncatedChannelName)
		}
	}

	// Check if we have too many channels
	if len(apiMsg.Channels) > app.MaxEventChannelsAtOnce {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Cannot broadcast to more than %d channels at once", app.MaxEventChannelsAtOnce))
		return fmt.Errorf("too many channels")
	}

	// Check the length of the event name
	if len(apiMsg.Name) > app.MaxEventNameLength {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Event name too long, max length is %d", app.MaxEventNameLength))
		return fmt.Errorf("event name too long")
	}

	// Check the size of the data payload
	if len(apiMsg.Data) > app.MaxEventPayloadInKb*1024 {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Event data too large, max size is %d KB", app.MaxEventPayloadInKb))
		return fmt.Errorf("event data too large")
	}
	return nil
}

// validateApiMessage checks the fields required to trigger an event.
func validateApiMessage(apiMsg *payloads.PusherApiMessage) error {
	switch {
	case apiMsg == nil:
		return errors.New("request body is required")
	case apiMsg.Name == "":
		return errors.New("name is required")
	case apiMsg.Data == "":
		return errors.New("data is required")
	case apiMsg.Channel == "" && len(apiMsg.Channels) == 0:
		return errors.New("channel or channels is required")
	}
	return nil
}

func broadcastMessage(message *payloads.PusherApiMessage, app *apps.App, server *Server) {
	// Publish to pubsub
	for _, channel := range message.Channels {
		log.Logger().Debugf("Broadcasting message to channel: %s", channel)
		_channelEvent := payloads.ChannelEvent{
			Event:    message.Name,
			Channel:  channel,
			Data:     message.Data,
			SocketID: message.SocketID,
		}
		channelEvent := _channelEvent.ToJson(false)

		_ = server.Adapter.Send(app.ID, channel, channelEvent, message.SocketID)

		if util.IsCacheChannel(channel) {
			cacheKey := cache.GetChannelCacheKey(app.ID, channel)
			server.CacheManager.SetEx(cacheKey, string(channelEvent), time.Duration(30)*time.Minute)
		}
	}
}

func EventTrigger(w http.ResponseWriter, r *http.Request, server *Server) {
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	var apiMsg *payloads.PusherApiMessage
	err := decodeJSON(r, &apiMsg)
	if err == nil {
		err = validateApiMessage(apiMsg)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		// Mark API message binding error in metrics
		server.MetricsManager.MarkError("api_message_binding_failed", app.ID)
		return
	}

	if err := checkMessageToBroadcast(w, app, apiMsg); err != nil {
		log.Logger().Infof("Error checking message to broadcast for appId(%s): %s", app.ID, err)
		// Mark API message failure in metrics
		server.MetricsManager.MarkError("api_message_validation_failed", app.ID)
		// the JSON response code is already handled in checkMessageToBroadcast on error
		// so we can just return here
		return
	}

	broadcastMessage(apiMsg, app, server)

	server.MetricsManager.MarkApiMessage(app.ID, apiMsg, struct {
		Ok bool `json:"ok"`
	}{Ok: true})

	// publish to pubsub
	// for _, Channel := range eventForm.Channels {
	// 	if !util.ValidChannel(Channel, 0) {
	// 		writeError(w, http.StatusBadRequest, util.ErrInvalidChannel.Error())
	// 		return
	// 	}
	// 	channelEvent := ChannelEvent{
	// 		Event:   eventForm.Name,
	// 		Channel: Channel,
	// 		Data:    eventForm.Data,
	// 		UserID:  eventForm.UserID,
	// 	}
	// 	err := hub.PublishChannelEventGlobally(channelEvent)
	//
	// 	if err != nil {
	// 		writeError(w, http.StatusInternalServerError, err.Error())
	// 		return
	// 	}
	// }
	writeEmptyJSON(w)
}

func BatchEventTrigger(w http.ResponseWriter, r *http.Request, server *Server) {
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	var batchEvents BatchEventForm
	err := decodeJSON(r, &batchEvents)
	if err == nil && batchEvents.Batch == nil {
		err = errors.New("batch is required")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	for _, apiMsg := range batchEvents.Batch {
		if err := checkMessageToBroadcast(w, app, apiMsg); err != nil {
			log.Logger().Infof("Error checking message to broadcast for appId(%s): %s", app.ID, err)
			return
		}

		broadcastMessage(apiMsg, app, server)
	}
	writeEmptyJSON(w)
}
