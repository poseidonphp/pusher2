package internal

import (
	"net/http"
	"slices"
	"strings"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/cache"

	"pusher/internal/constants"

	"pusher/internal/util"
	"pusher/log"
)

func ChannelIndex(w http.ResponseWriter, r *http.Request, server *Server) {
	filterByPrefix := r.URL.Query().Get("filter_by_prefix")
	info := r.URL.Query().Get("info")
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	log.Logger().Tracef("filter_by_prefix: %s", filterByPrefix)
	log.Logger().Tracef("info: %s", info)

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)

	// check if infoFields contains user_count
	getUserCount := slices.Contains(infoFields, "user_count")

	if filterByPrefix != "presence-" && getUserCount {
		writeError(w, http.StatusBadRequest, "info=user_count requires filter_by_prefix for presence channels")
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

	writeJSON(w, http.StatusOK, data)
}

func ChannelShow(w http.ResponseWriter, r *http.Request, server *Server) {
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	channel := constants.ChannelName(r.PathValue("channel_name"))
	info := r.URL.Query().Get("info")

	splitFn := func(c rune) bool {
		return c == ','
	}
	infoFields := strings.FieldsFunc(info, splitFn)

	data := &struct {
		pusherClient.Channel
		Cache string `json:"cache,omitempty"`
	}{}

	data.Name = string(channel)

	if slices.Contains(infoFields, "user_count") && util.IsPresenceChannel(channel) {
		data.UserCount = server.Adapter.GetChannelMembersCount(app.ID, channel, false)
		data.Occupied = data.UserCount > 0
	}

	if slices.Contains(infoFields, "subscription_count") {
		data.SubscriptionCount = int(server.Adapter.GetChannelSocketsCount(app.ID, channel, false))
		data.Occupied = data.SubscriptionCount > 0
	}

	if slices.Contains(infoFields, "cache") && util.IsCacheChannel(channel) {
		cacheKey := cache.GetChannelCacheKey(app.ID, channel)
		cachedData, cacheExists := server.CacheManager.Get(cacheKey)
		if cacheExists {
			data.Cache = cachedData
		}
	}

	writeJSON(w, http.StatusOK, data)
}

func ChannelUsers(w http.ResponseWriter, r *http.Request, server *Server) {
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	if r.PathValue("channel_name") == "" {
		writeError(w, http.StatusBadRequest, "Channel name required")
		return
	}

	channel := r.PathValue("channel_name")

	if !util.IsPresenceChannel(channel) {
		writeError(w, http.StatusBadRequest, "presence only")
		return
	}

	// _userIds := storage.PresenceChannelUserIDs(serverConfig.StorageManager, channel)
	members := server.Adapter.GetChannelMembers(app.ID, channel, false)

	data := pusherClient.Users{List: make([]pusherClient.User, 0)}

	for id, _ := range members {
		data.List = append(data.List, pusherClient.User{ID: id})
	}

	writeJSON(w, http.StatusOK, data)
}

func TerminateUserConnections(w http.ResponseWriter, r *http.Request, server *Server) {
	app, exists := getAppFromContext(r)
	if !exists {
		writeError(w, http.StatusInternalServerError, "App not found in context")
		return
	}

	userID := r.PathValue("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "Channel name required")
		return
	}
	server.Adapter.TerminateUserConnections(app.ID, userID)
	writeEmptyJSON(w)

}
