package internal

import (
	"github.com/gin-gonic/gin"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"
	"net/http"
	"pusher/internal/config"
	"pusher/internal/constants"
	"pusher/internal/storage"
	"pusher/internal/util"
	"pusher/log"
	"strings"
)

func ChannelIndex(c *gin.Context, serverConfig *config.ServerConfig) {
	filterByPrefix := c.Query("filter_by_prefix")
	info := c.Query("info")

	log.Logger().Tracef("filter_by_prefix: %s", filterByPrefix)
	log.Logger().Tracef("info: %s", info)

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)

	// check if infoFields contains user_count
	getUserCount := funk.Contains(infoFields, "user_count")

	if filterByPrefix != "presence-" && getUserCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "info=user_count requires filter_by_prefix for presence channels"})
		return
	}

	// channels := storage.Manager.Channels()
	channels := serverConfig.StorageManager.Channels()
	log.Logger().Tracef("Getting channels list: %v", channels)

	data := pusherClient.ChannelsList{
		Channels: make(map[string]pusherClient.ChannelListItem, len(channels)),
	}

	for _, channel := range channels {
		if filterByPrefix != "" && !strings.HasPrefix(string(channel), filterByPrefix) {
			log.Logger().Tracef("Skipping channel: %s", channel)
			continue
		}
		var count int
		if util.IsPresenceChannel(channel) {
			count = len(storage.PresenceChannelUserIDs(serverConfig.StorageManager, channel))
		} else {
			count = int(serverConfig.StorageManager.GetChannelCount(channel))
		}
		if count > 0 {
			item := &pusherClient.ChannelListItem{}
			if getUserCount {
				item.UserCount = count
			}
			data.Channels[string(channel)] = *item
		}
	}

	c.JSON(http.StatusOK, data)
}

func ChannelShow(c *gin.Context, serverConfig *config.ServerConfig) {
	channel := constants.ChannelName(c.Param("channel_name"))
	info := c.Query("info")

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)

	data := &struct {
		pusherClient.Channel
		Cache string `json:"cache,omitempty"`
	}{}

	data.Name = string(channel)

	subscriptionsCount := serverConfig.StorageManager.GetChannelCount(channel)
	data.Occupied = subscriptionsCount > 0

	if funk.Contains(infoFields, "user_count") && util.IsPresenceChannel(channel) {
		data.UserCount = len(storage.PresenceChannelUserIDs(serverConfig.StorageManager, channel))
	}

	if funk.Contains(infoFields, "subscription_count") {
		data.SubscriptionCount = int(subscriptionsCount)
	}

	if funk.Contains(infoFields, "cache") && util.IsCacheChannel(channel) {
		cachedData, exists := serverConfig.ChannelCacheManager.Get(string(channel))
		if exists {
			data.Cache = cachedData
		}
	}

	c.JSON(http.StatusOK, data)
}

func ChannelUsers(c *gin.Context, serverConfig *config.ServerConfig) {
	if c.Param("channel_name") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel name required"})
		return
	}

	channel := constants.ChannelName(c.Param("channel_name"))

	if !util.IsPresenceChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "presence only"})
		return
	}

	_userIds := storage.PresenceChannelUserIDs(serverConfig.StorageManager, channel)

	data := pusherClient.Users{List: make([]pusherClient.User, 0)}

	for _, id := range _userIds {
		data.List = append(data.List, pusherClient.User{ID: id})
	}

	c.JSON(http.StatusOK, data)
}
