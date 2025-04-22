package internal

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/thoas/go-funk"
	"pusher/internal/cache"

	"pusher/internal/constants"

	"pusher/internal/util"
	"pusher/log"
)

func ChannelIndex(c *gin.Context, server *Server) {
	filterByPrefix := c.Query("filter_by_prefix")
	info := c.Query("info")
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

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

	var channels map[constants.ChannelName]int64
	// channels := storage.Manager.Channels()
	// channels := server.Adapter.GetChannelsWithSocketsCount(app.ID, false)
	if filterByPrefix == "presence-" && getUserCount {
		channels = server.Adapter.GetPresenceChannelsWithUsersCount(app.ID, false)
	} else {
		channels = server.Adapter.GetChannelsWithSocketsCount(app.ID, false)
	}

	log.Logger().Tracef("Getting channels list: %v", channels)

	data := pusherClient.ChannelsList{
		Channels: make(map[string]pusherClient.ChannelListItem),
	}

	for channelName, channelCount := range channels {
		if filterByPrefix != "" && !strings.HasPrefix(channelName, filterByPrefix) {
			log.Logger().Tracef("Skipping Channel: %s", channelName)
			continue
		}

		item := &pusherClient.ChannelListItem{}
		if getUserCount {
			item.UserCount = int(channelCount)
		}
		data.Channels[channelName] = *item

	}

	c.JSON(http.StatusOK, data)
}

func ChannelShow(c *gin.Context, server *Server) {
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

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

	if funk.Contains(infoFields, "user_count") && util.IsPresenceChannel(channel) {
		data.UserCount = server.Adapter.GetChannelMembersCount(app.ID, channel, false)
		data.Occupied = data.UserCount > 0
	}

	if funk.Contains(infoFields, "subscription_count") {
		data.SubscriptionCount = int(server.Adapter.GetChannelSocketsCount(app.ID, channel, false))
		data.Occupied = data.SubscriptionCount > 0
	}

	if funk.Contains(infoFields, "cache") && util.IsCacheChannel(channel) {
		cacheKey := cache.GetChannelCacheKey(app.ID, channel)
		cachedData, cacheExists := server.CacheManager.Get(cacheKey)
		if cacheExists {
			data.Cache = cachedData
		}
	}

	c.JSON(http.StatusOK, data)
}

func ChannelUsers(c *gin.Context, server *Server) {
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

	if c.Param("channel_name") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel name required"})
		return
	}

	channel := c.Param("channel_name")

	if !util.IsPresenceChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "presence only"})
		return
	}

	// _userIds := storage.PresenceChannelUserIDs(serverConfig.StorageManager, channel)
	members := server.Adapter.GetChannelMembers(app.ID, channel, false)

	data := pusherClient.Users{List: make([]pusherClient.User, 0)}

	for id, _ := range members {
		data.List = append(data.List, pusherClient.User{ID: id})
	}

	c.JSON(http.StatusOK, data)
}

func TerminateUserConnections(c *gin.Context, server *Server) {
	app, exists := getAppFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
		c.Abort()
		return
	}

	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel name required"})
		return
	}
	server.Adapter.TerminateUserConnections(app.ID, userID)
	c.JSON(http.StatusOK, gin.H{})

}
