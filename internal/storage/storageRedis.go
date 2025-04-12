package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/redis/go-redis/v9"
	"pusher/internal/constants"
	"pusher/internal/payloads"
	"pusher/internal/util"
	"pusher/log"
)

type RedisStorage struct {
	Client    redis.UniversalClient
	KeyPrefix string
}

// *********** NON INTERFACE METHODS ***********

func (r *RedisStorage) getKey(key string) string {
	if r.KeyPrefix == "" {
		return key
	}
	return r.KeyPrefix + ":" + key
}

func (r *RedisStorage) nodeListKey() string {
	return r.getKey("hub:nodes")
}

func (r *RedisStorage) channelListKey() string {
	return r.getKey("hub:channels")
}

// returns the redis key for a presence channel (ie: <prefix>:channels:presence-<channel name>)
func (r *RedisStorage) presenceChannelKey(channelName constants.ChannelName) string {
	return r.getKey(fmt.Sprintf("channels:%s", channelName))
}

func (r *RedisStorage) channelCountKey(nodeID constants.NodeID) string {
	return r.getKey(fmt.Sprintf("channel_counts:%s", nodeID))
}

// *********** INTERFACE-SPECIFIC METHODS ***********

// Init Initialize the storage manager
func (r *RedisStorage) Init() error {
	if r.Client == nil {
		return fmt.Errorf("redis client is not initialized in storageRedis")
	}
	if r.KeyPrefix == "" {
		return fmt.Errorf("key prefix is not set in storageRedis")
	}
	return nil
}

// Start the storage manager
func (r *RedisStorage) Start() {}

func (r *RedisStorage) AddNewNode(nodeID constants.NodeID) error {
	log.Logger().Tracef("Adding new node %s", nodeID)
	currentTimestampInEpoch := float64(time.Now().Unix())
	r.Client.ZAdd(context.Background(), r.nodeListKey(), redis.Z{Score: currentTimestampInEpoch, Member: string(nodeID)})
	return nil
}

func (r *RedisStorage) RemoveNode(nodeID constants.NodeID) error {
	r.Client.ZRem(context.Background(), r.nodeListKey(), string(nodeID))
	return nil
}

// PurgeNodeData removes a node from the list of nodes, and clears any channel data associated with the node.
// This process can be time-consuming if there are many channels (ie one per user with a lot of users).
func (r *RedisStorage) PurgeNodeData(nodeID constants.NodeID) error {
	// first remove any channel counts for this node:
	log.Logger().Tracef("   ...... Deleting %s", r.channelCountKey(nodeID))
	r.Client.Del(context.Background(), r.channelCountKey(nodeID))

	// now let's try to remove any presence channel data associated with this node
	log.Logger().Tracef("   ...... Getting keys matching %s", r.presenceChannelKey("*"))
	presenceChannelKeys := r.Client.Keys(context.Background(), r.presenceChannelKey("*")).Val()

	for _, channelKey := range presenceChannelKeys {
		cursor := uint64(0)

		for {
			log.Logger().Tracef("   ............ Scanning %s for %s", channelKey, fmt.Sprintf("node:%s:socket:*", nodeID))
			// get all keys within the channel key, for the node
			nodeKeys, cursor, err := r.Client.HScan(context.Background(), channelKey, cursor, fmt.Sprintf("node:%s:socket:*", nodeID), 0).Result()
			if err != nil {
				log.Logger().Errorf("Error scanning channel %s: %s", channelKey, err)
				break
			}

			for i, nodeKey := range nodeKeys {
				if i%2 != 0 {
					continue
				}

				log.Logger().Tracef("   .................. Deleting hash key %s", nodeKey)
				r.Client.HDel(context.Background(), channelKey, nodeKey)
			}

			if cursor == 0 || len(nodeKeys) == 0 {
				break
			}
		}
	}

	// lastly, remove the node from the list of nodes
	return r.RemoveNode(nodeID)
}

func (r *RedisStorage) GetAllNodes() ([]string, error) {
	nodes, err := r.Client.ZRange(context.Background(), r.nodeListKey(), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *RedisStorage) SendNodeHeartbeat(nodeID constants.NodeID) *time.Time {
	currentTime := time.Now()
	currentTimestampInEpoch := float64(currentTime.Unix())
	r.Client.ZAdd(context.Background(), r.nodeListKey(), redis.Z{Score: currentTimestampInEpoch, Member: string(nodeID)})
	return &currentTime
}

func (r *RedisStorage) AddChannel(channel constants.ChannelName) error {
	return r.Client.SAdd(context.Background(), r.channelListKey(), string(channel)).Err()
}

func (r *RedisStorage) RemoveChannel(channel constants.ChannelName) error {
	// remove channel from the list of channels
	_ = r.Client.SRem(context.Background(), r.channelListKey(), string(channel)).Err()

	// look for other channel keys that may need to be removed
	if util.IsPresenceChannel(channel) {
		r.Client.Del(context.Background(), r.presenceChannelKey(channel))
	} else {
		// scan for each node's channel_count set, and remove the channel from each
		nodes := r.Client.ZRange(context.Background(), r.nodeListKey(), 0, -1).Val()
		for _, node := range nodes {
			r.Client.HDel(context.Background(), r.channelCountKey(constants.NodeID(node)), string(channel))
		}
	}
	// TODO: Send vacated webhook
	return nil
}

func (r *RedisStorage) Channels() []constants.ChannelName {
	members := r.Client.SMembers(context.Background(), r.channelListKey()).Val()
	channels := make([]constants.ChannelName, len(members))

	for i, member := range members {
		channels[i] = constants.ChannelName(member)
	}
	return channels
}

func (r *RedisStorage) AdjustChannelCount(nodeID constants.NodeID, channelName constants.ChannelName, countToAdd int64) (newCount int64, err error) {
	/*
		•	Key: presence:channel_counts:{nodeId}
		•	Type: Hash
		•	Fields: each field is {channelName}, with the integer count as the value.
	*/
	channelCountKey := r.channelCountKey(nodeID)
	newCountAfterIncr := r.Client.HIncrBy(context.Background(), channelCountKey, string(channelName), countToAdd)
	if newCountAfterIncr.Err() != nil {
		log.Logger().Errorf("Error incrementing channel count: %s", newCountAfterIncr.Err())
		return 0, newCountAfterIncr.Err()
	}
	if newCountAfterIncr.Val() == 0 {
		// if the count is 0, remove the channel from the hash
		r.Client.HDel(context.Background(), channelCountKey, string(channelName))
	}
	newCount = newCountAfterIncr.Val()

	return
}

func (r *RedisStorage) GetChannelCount(channelName constants.ChannelName) int64 {
	if util.IsPresenceChannel(channelName) {
		channelKey := r.presenceChannelKey(channelName)
		count, err := r.Client.HLen(context.Background(), channelKey).Result()
		if err != nil {
			return 0
		}
		return count
	}

	nodes := r.Client.ZRange(context.Background(), r.nodeListKey(), 0, -1).Val()
	runningCount := int64(0)

	for _, node := range nodes {
		count, err := r.Client.HGet(context.Background(), r.channelCountKey(constants.NodeID(node)), string(channelName)).Int64()
		if err != nil {
			continue
		}
		runningCount += count
	}

	return runningCount
}

func (r *RedisStorage) AddUserToPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID, memberData pusherClient.MemberData) error {
	channelKey := r.presenceChannelKey(channelName)

	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)

	memberDataBytes, err := json.Marshal(memberData)
	if err != nil {
		log.Logger().Errorf("Error marshalling member data: %s", err)
		return fmt.Errorf("error marshalling member data: %w", err)
	}

	r.Client.HSet(context.Background(), channelKey, memberValue, string(memberDataBytes))

	// Note: we do not adjust the channel count for presence channels. When the count is needed, we just return the length of the hash.
	return nil
}

func (r *RedisStorage) RemoveUserFromPresence(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) error {
	channelKey := r.presenceChannelKey(channelName)
	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)
	r.Client.HDel(context.Background(), channelKey, memberValue)
	return nil
}

// GetPresenceData returns the presence data for a given channel and socketIDs for the current user (list of users and their data).
func (r *RedisStorage) GetPresenceData(channelName constants.ChannelName, currentUser pusherClient.MemberData) ([]byte, []constants.SocketID, error) {
	_presenceData := payloads.PresenceData{
		IDs:   []string{},
		Hash:  map[string]map[string]string{},
		Count: 0,
	}

	channelKey := r.presenceChannelKey(channelName)
	members, err := r.Client.HGetAll(context.Background(), channelKey).Result()
	usersSocketIDs := make([]constants.SocketID, 0)
	if err != nil {
		log.Logger().Errorf("Error getting presence channel members: %s", err)
		return nil, usersSocketIDs, err
	}

	// append the current user, since they are likely not in the list yet
	if currentUser.UserID != "" {
		_presenceData.Hash[currentUser.UserID] = currentUser.UserInfo
		_presenceData.IDs = append(_presenceData.IDs, currentUser.UserID)
	}

	for redisKey, member := range members {
		var memberData pusherClient.MemberData
		mErr := json.Unmarshal([]byte(member), &memberData)
		if mErr != nil {
			log.Logger().Errorf("Error unmarshalling member data: %s", mErr)
			return nil, usersSocketIDs, mErr
		}
		if memberData.UserID == currentUser.UserID {
			// get the last section of the redis key, as that is the socket id
			parts := strings.Split(redisKey, ":")
			socketId := parts[len(parts)-1]
			usersSocketIDs = append(usersSocketIDs, constants.SocketID(socketId))
		}
		_presenceData.Hash[memberData.UserID] = memberData.UserInfo
		_presenceData.IDs = append(_presenceData.IDs, memberData.UserID)
	}

	_presenceData.Count = len(_presenceData.IDs)

	presenceData, pErr := json.Marshal(map[string]payloads.PresenceData{"presence": _presenceData})
	if pErr != nil {
		log.Logger().Errorf("Error marshalling presence data: %s", pErr)
		return nil, usersSocketIDs, pErr
	}
	return presenceData, usersSocketIDs, nil
}

func (r *RedisStorage) GetPresenceDataForSocket(nodeID constants.NodeID, channelName constants.ChannelName, socketID constants.SocketID) (*pusherClient.MemberData, error) {
	channelKey := r.presenceChannelKey(channelName)
	memberValue := fmt.Sprintf("node:%s:socket:%s", nodeID, socketID)

	memberData, err := r.Client.HGet(context.Background(), channelKey, memberValue).Result()
	if err != nil {
		return nil, err
	}

	var member pusherClient.MemberData

	mErr := json.Unmarshal([]byte(memberData), &member)
	if mErr != nil {
		return nil, mErr
	}

	return &member, nil
}

func (r *RedisStorage) SocketDidHeartbeat(_ constants.NodeID, _ constants.SocketID, _ map[constants.ChannelName]constants.ChannelName) error {
	return nil
}

func (r *RedisStorage) Cleanup() {
	// get nodes from list that have not sent a heartbeat in the last 15 seconds
	oldNodes, zErr := r.Client.ZRangeByScore(context.Background(), r.nodeListKey(), &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", time.Now().Add(-15*time.Second).Unix()),
	}).Result()
	if zErr != nil {
		log.Logger().Errorf("Error getting old nodes: %s", zErr)
		return
	}

	for _, node := range oldNodes {
		log.Logger().Tracef("Purging node %s", node)
		nodeID := constants.NodeID(node)
		_ = r.PurgeNodeData(nodeID)
	}
}
