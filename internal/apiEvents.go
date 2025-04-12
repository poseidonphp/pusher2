package internal

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"pusher/internal/constants"
	"pusher/internal/util"
)

var maxTriggerableChannels = 100

// EventForm ...
type EventForm struct {
	Name     string                  `form:"name" json:"name" binding:"required"`
	Data     string                  `form:"data" json:"data" binding:"required"` // limit to 10kb
	Channels []constants.ChannelName `form:"channels" json:"channels,omitempty"`  // limit to 100 channel
	Channel  constants.ChannelName   `form:"channel" json:"channel,omitempty"`
	SocketID constants.SocketID      `form:"socket_id" json:"socket_id,omitempty"` // excludes the event from being sent to
	UserID   string                  `json:"user_id,omitempty"`                    // optional, present only if this is a `client event` on a `presence channel`
}

// BatchEventForm ...
type BatchEventForm struct {
	Batch []EventForm `form:"batch" json:"batch" binding:"required"`
}

// func EventTrigger(c *gin.Context) {
func EventTrigger(c *gin.Context, hub *Hub) {
	var eventForm EventForm
	if err := c.ShouldBindJSON(&eventForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// limit to 10kb
	if len(eventForm.Data) > constants.MaxEventSizeKb*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Event data too large"})
		return
	}

	if len(eventForm.Channel) == 0 && len(eventForm.Channels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing channel params"})
		return
	}

	// Check if channel is present and channels is empty; add channel to channels
	if len(eventForm.Channel) > 0 && len(eventForm.Channels) == 0 {
		eventForm.Channels = append(eventForm.Channels, eventForm.Channel)
	}

	// limit to 100
	if len(eventForm.Channels) > maxTriggerableChannels {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": util.ErrAPIReqEventChannelsSizeTooLong.Error()})
		return
	}

	// publish to pubsub
	for _, channel := range eventForm.Channels {
		if !util.ValidChannel(channel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": util.ErrInvalidChannel.Error()})
			return
		}
		channelEvent := ChannelEvent{
			Event:   eventForm.Name,
			Channel: channel,
			Data:    eventForm.Data,
			UserID:  eventForm.UserID,
		}
		err := hub.PublishChannelEventGlobally(channelEvent)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{})
}

func BatchEventTrigger(c *gin.Context, hub *Hub) {
	var batchEvents BatchEventForm
	if err := c.ShouldBindJSON(&batchEvents); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, eventForm := range batchEvents.Batch {
		// limit to 10kb
		if len(eventForm.Data) > constants.MaxEventSizeKb*1024 {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Event data too large"})
			return
		}

		if len(eventForm.Channel) == 0 && len(eventForm.Channels) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing channel params"})
			return
		}

		// Check if channel is present and channels is empty; add channel to channels
		if len(eventForm.Channel) > 0 && len(eventForm.Channels) == 0 {
			eventForm.Channels = append(eventForm.Channels, eventForm.Channel)
		}

		// limit to 100
		if len(eventForm.Channels) > maxTriggerableChannels {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": util.ErrAPIReqEventChannelsSizeTooLong.Error()})
			return
		}

		for _, channel := range eventForm.Channels {
			if !util.ValidChannel(channel) {
				c.JSON(http.StatusBadRequest, gin.H{"error": util.ErrInvalidChannel.Error()})
				return
			}
			channelEvent := ChannelEvent{
				Event:   eventForm.Name,
				Channel: channel,
				Data:    eventForm.Data,
				UserID:  eventForm.UserID,
			}
			err := hub.PublishChannelEventGlobally(channelEvent)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{})
}
