package internal

import (
	"fmt"
	"net/http"
	"time"

	"pusher/internal/apps"
	"pusher/internal/cache"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"

	"github.com/gin-gonic/gin"
)

// var maxTriggerableChannels = 100

// BatchEventForm ...
type BatchEventForm struct {
	Batch []*payloads.PusherApiMessage `form:"batch" json:"batch" binding:"required"`
}

func checkMessageToBroadcast(c *gin.Context, app *apps.App, apiMsg *payloads.PusherApiMessage) error {
	if apiMsg.Channel != "" && (apiMsg.Channels == nil || len(apiMsg.Channels) == 0) {
		apiMsg.Channels = []constants.ChannelName{apiMsg.Channel}
	}

	// Check if the Channel is valid
	for _, channel := range apiMsg.Channels {
		if !util.ValidChannel(channel, app.MaxChannelNameLength) {
			c.JSON(http.StatusBadRequest, gin.H{"error": util.ErrInvalidChannel.Error()})
			return fmt.Errorf("invalid Channel: %s", channel)
		}
	}

	// Check if we have too many channels
	if len(apiMsg.Channels) > app.MaxEventChannelsAtOnce {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Cannot broadcast to more than %d channels at once", app.MaxEventChannelsAtOnce)})
		return fmt.Errorf("too many channels")
	}

	// Check the length of the event name
	if len(apiMsg.Name) > app.MaxEventNameLength {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Event name too long, max length is %d", app.MaxEventNameLength)})
		return fmt.Errorf("event name too long")
	}

	// Check the size of the data payload
	if len(apiMsg.Data) > app.MaxEventPayloadInKb*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Event data too large, max size is %d KB", app.MaxEventPayloadInKb)})
		return fmt.Errorf("event data too large")
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

func EventTrigger(c *gin.Context, server *Server) {
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

	var apiMsg *payloads.PusherApiMessage
	if err := c.ShouldBind(&apiMsg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		// Mark API message binding error in metrics
		server.MetricsManager.MarkError("api_message_binding_failed", app.ID)
		return
	}

	if err := checkMessageToBroadcast(c, app, apiMsg); err != nil {
		log.Logger().Infof("Error checking message to broadcast for appId(%s): %s", app.ID, err)
		// Mark API message failure in metrics
		server.MetricsManager.MarkError("api_message_validation_failed", app.ID)
		return
	}

	broadcastMessage(apiMsg, app, server)

	server.MetricsManager.MarkApiMessage(app.ID, apiMsg, struct {
		Ok bool `json:"ok"`
	}{Ok: true})

	// publish to pubsub
	// for _, Channel := range eventForm.Channels {
	// 	if !util.ValidChannel(Channel, 0) {
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": util.ErrInvalidChannel.Error()})
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
	// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 		return
	// 	}
	// }
	c.JSON(http.StatusOK, gin.H{})
}

func BatchEventTrigger(c *gin.Context, server *Server) {
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

	var batchEvents BatchEventForm
	if err := c.ShouldBindJSON(&batchEvents); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, apiMsg := range batchEvents.Batch {
		if err := checkMessageToBroadcast(c, app, apiMsg); err != nil {
			log.Logger().Infof("Error checking message to broadcast for appId(%s): %s", app.ID, err)
			return
		}

		broadcastMessage(apiMsg, app, server)
	}
	c.JSON(http.StatusOK, gin.H{})
}
