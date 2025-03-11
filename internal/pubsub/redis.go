package pubsub

import (
	"encoding/json"
	"github.com/go-redis/redis"
	"pusher/internal/constants"
	"pusher/log"
)

//var (
//	redisClient redis.UniversalClient
//	redisOnce   sync.Once
//)

type RedisPubSubManager struct {
	PubSubCore
	Client redis.UniversalClient
	//Client clients.RedisClient
}

//func (r *RedisPubSubManager) initRedisClient(redisURL string) error {
//	redisOptions, err := redis.ParseURL(redisURL)
//	if err != nil {
//		return err
//	}
//	universalOptions := &redis.UniversalOptions{
//		Addrs:       []string{redisOptions.Addr},
//		DB:          redisOptions.DB,
//		Password:    redisOptions.Password,
//		PoolSize:    redisOptions.PoolSize,
//		PoolTimeout: redisOptions.PoolTimeout,
//	}
//	if env.GetBool("USE_TLS", false) {
//		universalOptions.TLSConfig = &tls.Config{
//			MinVersion: tls.VersionTLS12,
//		}
//	}
//	redisClient = redis.NewUniversalClient(universalOptions)
//	r.Client = redisClient
//
//	_, err = redisClient.Ping().Result()
//	if err != nil {
//		return err
//	}
//	return nil
//}
//
//func (r *RedisPubSubManager) initClusterRedisClient(redisURLs []string) error {
//	var err error
//	var nodes []string
//	var redisOptions *redis.Options
//	for _, redisURL := range redisURLs {
//		redisOptions, err = redis.ParseURL(redisURL)
//		if err != nil {
//			log.Logger().Panic(err)
//		}
//		nodes = append(nodes, redisOptions.Addr)
//	}
//
//	universalOptions := &redis.UniversalOptions{
//		Addrs:       nodes,
//		DB:          redisOptions.DB,
//		Password:    redisOptions.Password,
//		PoolSize:    redisOptions.PoolSize,
//		PoolTimeout: redisOptions.PoolTimeout,
//	}
//	if env.GetBool("USE_TLS", false) {
//		universalOptions.TLSConfig = &tls.Config{
//			MinVersion: tls.VersionTLS12,
//		}
//	}
//	redisClient = redis.NewUniversalClient(universalOptions)
//	r.Client = redisClient
//	_, err = redisClient.Ping().Result()
//	if err != nil {
//		return err
//	}
//	return nil
//}
//
//func (r *RedisPubSubManager) Init(prefix string) error {
//	if redisClient == nil {
//		redisOnce.Do(func() {
//			redisURLs := strings.Split(env.GetString("REDIS_URL", "redis://localhost:6379"), ",")
//			if len(redisURLs) > 1 || env.GetBool("REDIS_CLUSTER", false) {
//				err := r.initClusterRedisClient(redisURLs)
//				if err != nil {
//					log.Logger().Fatal(err)
//				}
//			} else {
//				err := r.initRedisClient(redisURLs[0])
//				if err != nil {
//					log.Logger().Fatal(err)
//				}
//			}
//		})
//	}
//	r.keyPrefix = prefix
//	return nil
//}

func (r *RedisPubSubManager) SetKeyPrefix(prefix string) {
	r.keyPrefix = prefix
}

func (r *RedisPubSubManager) Publish(channelName constants.ChannelName, message ServerMessage) error {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Logger().Errorf("Could not marshal message: %s", message.Event)
		return err
	}
	r.Client.Publish(string(channelName), string(msgBytes))
	return nil
}

func (r *RedisPubSubManager) Subscribe(channelName constants.ChannelName, receiveChannel chan<- ServerMessage) {
	s := r.Client.Subscribe(string(channelName))
	if s == nil {
		log.Logger().Error("Could not subscribe to Redis channel")
		return
	}
	defer func() {
		log.Logger().Info("Closing Redis subscription")
		_ = s.Close()
	}()
	for {
		msgI, err := s.Receive()
		if err != nil {
			log.Logger().Error(err)
			return
		}
		switch msg := msgI.(type) {
		case *redis.Message:
			var sMsg ServerMessage
			umErr := json.Unmarshal([]byte(msg.Payload), &sMsg)
			if umErr != nil {
				log.Logger().Errorf("Could not unmarshal message: %v", umErr)
				continue
			}
			receiveChannel <- sMsg
		case *redis.Subscription:
			log.Logger().Infof("Subscription message: %s %s %d", msg.Channel, msg.Kind, msg.Count)
		default:
			log.Logger().Errorf("Unknown message type: %v", msg)
		}
	}
}

//
//func (r *RedisPubSubManager) AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error {
//	/*
//		•	Key: presence:channels:{channelName}
//		•	Type: Hash
//		•	Members: node:{nodeId}:socket:{socketId} => {memberData}
//	*/
//	// HSET presence:channel:chat-room node:node1:socket:ABC '{"user_id":123,"name":"Alice"}'
//	channelKey := r.presenceChannelKey(channelName)
//	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)
//	memberDataBytes, err := json.Marshal(memberData)
//	if err != nil {
//		log.Logger().Errorf("Error marshalling member data: %s", err)
//		return fmt.Errorf("error marshalling member data: %w", err)
//	}
//	r.Client.HSet(channelKey, memberValue, string(memberDataBytes))
//	// Note: we do not adjust the channel count for presence channels. When the count is needed, we just return the length of the hash.
//	return nil
//}
//
//func (r *RedisPubSubManager) RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error {
//	channelKey := r.presenceChannelKey(channelName)
//	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)
//	r.Client.HDel(channelKey, memberValue)
//	return nil
//}
//
//func (r *RedisPubSubManager) AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) error {
//	/*
//		•	Key: presence:channel_counts:{nodeId}
//		•	Type: Hash
//		•	Fields: each field is {channelName}, with the integer count as the value.
//	*/
//	// HINCRBY presence:channel_counts:{nodeId} {channelName} 1
//	channelCountKey := r.channelCountKey(channelName)
//	r.Client.HIncrBy(channelCountKey, string(nodeID), countToAdd)
//	return nil
//}
//
//func (r *RedisPubSubManager) GetPresenceData(channelName constants.ChannelName) ([]byte, error) {
//	_presenceData := PresenceData{
//		IDs:   []string{},
//		Hash:  map[string]map[string]string{},
//		Count: 0,
//	}
//
//	// Get all members from redish with HGETALL
//	channelKey := r.presenceChannelKey(channelName)
//	members, err := r.Client.HGetAll(channelKey).Result()
//	if err != nil {
//		log.Logger().Errorf("Error getting presence channel members: %s", err)
//		return nil, err
//	}
//	for _, member := range members {
//		var memberData pusherClient.MemberData
//		mErr := json.Unmarshal([]byte(member), &memberData)
//		if mErr != nil {
//			log.Logger().Errorf("Error unmarshalling member data: %s", mErr)
//			return nil, mErr
//		}
//		_presenceData.Hash[memberData.UserID] = memberData.UserInfo
//		_presenceData.IDs = append(_presenceData.IDs, memberData.UserID)
//	}
//
//	_presenceData.Count = len(_presenceData.IDs)
//
//	presenceData, pErr := json.Marshal(map[string]PresenceData{"presence": _presenceData})
//	if pErr != nil {
//		log.Logger().Errorf("Error marshalling presence data: %s", pErr)
//		return nil, pErr
//	}
//	return presenceData, nil
//}
//
//func (r *RedisPubSubManager) GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error) {
//	channelKey := r.presenceChannelKey(channelName)
//	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)
//	memberData, err := r.Client.HGet(channelKey, memberValue).Result()
//	if err != nil {
//		return nil, err
//	}
//	var member pusherClient.MemberData
//	mErr := json.Unmarshal([]byte(memberData), &member)
//	if mErr != nil {
//		return nil, mErr
//	}
//	return &member, nil
//}
//
//func (r *RedisPubSubManager) GetChannelCount(channelName constants.ChannelName) int64 {
//	if IsPresenceChannel(channelName) {
//		channelKey := r.presenceChannelKey(channelName)
//		count, err := r.Client.HLen(channelKey).Result()
//		if err != nil {
//			return 0
//		}
//		return count
//	}
//	channelCountKey := r.channelCountKey(channelName)
//	//count, err := r.Client.HGet(channelCountKey, string(channelName)).Int64()
//	nodes, err := r.Client.HGetAll(channelCountKey).Result()
//	runningCount := 0
//	for _, count := range nodes {
//		c, _ := strconv.Atoi(count)
//		runningCount += c
//	}
//	if err != nil {
//		return 0
//	}
//	return int64(runningCount)
//}

//func (r *RedisPubSubManager) RemoveEmptyChannel(channelName constants.ChannelName) {
//	var channelKey string
//	if IsPresenceChannel(channelName) {
//		channelKey = r.presenceChannelKey(channelName)
//	}
//	channelKey = r.channelCountKey(channelName)
//	r.Client.Del(channelKey)
//}

//func (r *RedisPubSubManager) AddNewNode(nodeID constants.NodeID) error {
//	// ZADD hub:nodes [expiryTimestamp 30 seconds past current time] node:{nodeId}
//
//	// set the expiryTimestamp to an epoch time of 30 seconds from now
//	expiryTimestamp := float64(time.Now().Add(30 * time.Second).Unix())
//	r.Client.ZAdd(r.nodeRegistrationKey(), redis.Z{
//		Score:  expiryTimestamp,
//		Member: "node:" + string(nodeID),
//	})
//	return nil
//}
