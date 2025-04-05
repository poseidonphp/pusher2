package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"pusher/internal/constants"
	"pusher/internal/storage"
	"pusher/internal/util"
	"strings"
)

func ChannelIndex(c *gin.Context) {
	filterByPrefix := c.Query("filter_by_prefix")
	info := c.Query("info")

	var selectedChannels []constants.ChannelName
	channels := storage.Manager.Channels()
	for _, channel := range channels {
		if storage.Manager.GetChannelCount(channel) > 0 {
			// occupied channels
			if filterByPrefix != "" && strings.HasPrefix(string(channel), filterByPrefix) {
				selectedChannels = append(selectedChannels, channel)
			} else {
				selectedChannels = append(selectedChannels, channel)
			}
		}
	}

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)
	data := make(map[constants.ChannelName]map[string]any)
	for _, channel := range selectedChannels {
		if len(infoFields) > 0 {
			for _, infoField := range infoFields {
				if infoField == "user_count" && util.IsPresenceChannel(channel) {
					data[channel] = map[string]any{"user_count": len(storage.PresenceChannelUserIDs(channel))}
				}
			}
		} else {
			data[channel] = make(map[string]any)
		}
	}

	c.JSON(http.StatusOK, gin.H{"channels": data})
}

func ChannelShow(c *gin.Context) {
	channel := constants.ChannelName(c.Param("channel_name"))
	info := c.Query("info")

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)

	data := make(map[string]interface{})
	subscriptionsCount := storage.Manager.GetChannelCount(channel)
	data["occupied"] = subscriptionsCount > 0
	for _, infoField := range infoFields {
		if infoField == "user_count" && util.IsPresenceChannel(channel) {
			data["user_count"] = len(storage.PresenceChannelUserIDs(channel))
		}
		if infoField == "subscription_count" {
			data["subscription_count"] = subscriptionsCount
		}
	}

	c.JSON(http.StatusOK, data)
}

func ChannelUsers(c *gin.Context) {
	if c.Param("channel_name") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel name required"})
		return
	}

	channel := constants.ChannelName(c.Param("channel_name"))

	if !util.IsPresenceChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "presence only"})
		return
	}

	_userIds := storage.PresenceChannelUserIDs(channel)
	userIds := make([]map[string]string, len(_userIds))
	for index, id := range _userIds {
		userIds[index] = map[string]string{"ID": id}
	}

	c.JSON(http.StatusOK, gin.H{"users": userIds})
}
