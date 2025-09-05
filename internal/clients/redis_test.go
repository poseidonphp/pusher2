package clients

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	// "github.com/go-redis/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// setupMiniRedis creates a new miniredis server and returns it with a connected UniversalClient
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %v", err)
	}

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{mr.Addr()},
	})

	// Verify connection is working
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		t.Fatalf("Could not connect to miniredis: %v", err)
	}

	return mr, client
}

func TestRedis_InitRedis(t *testing.T) {
	mr1, client1 := setupMiniRedis(t)
	defer mr1.Close()
	defer client1.Close()

	add1 := fmt.Sprintf("redis://%v", mr1.Addr())

	rc := &RedisClient{}

	err := rc.InitRedis(add1, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, rc.Client)

	// Test redis url without the port
	err = rc.InitRedis("redis://"+mr1.Host(), false, false)
	assert.NoError(t, err)
}

func TestRedis_InitRedisCluster(t *testing.T) {
	mr1, client1 := setupMiniRedis(t)
	defer mr1.Close()
	defer client1.Close()

	mr2, client2 := setupMiniRedis(t)
	defer mr2.Close()
	defer client2.Close()

	add1 := fmt.Sprintf("redis://%v", mr1.Addr())
	add2 := fmt.Sprintf("redis://%v", mr1.Addr())

	rc := &RedisClient{}
	err := rc.InitRedis(strings.Join([]string{add1, add2}, ","), true, false)

	assert.NoError(t, err)
	assert.NotNil(t, rc.Client)
}

func TestRedis_InitRedis_BadURL(t *testing.T) {
	rc := &RedisClient{}

	mr1, client1 := setupMiniRedis(t)
	defer mr1.Close()
	defer client1.Close()

	// Test redis url without the leading "redis://"
	err := rc.InitRedis(mr1.Addr(), false, false)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to parse redis url")
	}

	err = rc.InitRedis("badredisurl", false, false)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to parse redis url")
	}

	err = rc.InitRedis("badredisurl,anotherbadurl", true, false)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to parse redis url")
	}

	// Test one good and one bad url in cluster mode
	err = rc.InitRedis("redis://localhost:6379,anotherbadurl", true, false)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to parse redis url")
	}
}

func TestRedis_parseUrlWithTLS(t *testing.T) {
	tlsAddress := "rediss://localhost:6379"
	rc := &RedisClient{}
	opts, err := rc.ParseUrlAndGetOptions(tlsAddress, false)
	assert.NoError(t, err)
	assert.NotNil(t, opts)
	assert.NotNil(t, opts.TLSConfig)
	assert.NotZero(t, opts.TLSConfig.MinVersion)
	assert.GreaterOrEqual(t, opts.TLSConfig.MinVersion, uint16(771)) // 771 is tls.VersionTLS11 (1.1)

	tlsAddress = "redis://localhost:6379"
	rc = &RedisClient{}
	opts, err = rc.ParseUrlAndGetOptions(tlsAddress, true)
	assert.NoError(t, err)
	assert.NotNil(t, opts)
	assert.NotNil(t, opts.TLSConfig)
	assert.NotZero(t, opts.TLSConfig.MinVersion)
	assert.GreaterOrEqual(t, opts.TLSConfig.MinVersion, uint16(771)) // 771 is tls.VersionTLS11 (1.1)

}

func TestRedis_parseUrlWithOutTLS(t *testing.T) {
	tlsAddress := "redis://localhost:6379"
	rc := &RedisClient{}
	opts, err := rc.ParseUrlAndGetOptions(tlsAddress, false)
	assert.NoError(t, err)
	assert.NotNil(t, opts)
	assert.Nil(t, opts.TLSConfig)
}

func TestRedis_InitRedis_PingFail(t *testing.T) {
	rc := &RedisClient{}
	err := rc.InitRedis("redis://localhost:6390", false, false) // Assuming nothing is running on 6390
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "connect: connection refused")
	}

	rc = &RedisClient{}
	err = rc.InitRedis("redis://localhost:6390,redis://localhost:6391", true, false) // Assuming nothing is running on 6390
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "connect: connection refused")
	}

}

func TestRedis_GetKey(t *testing.T) {
	rc := &RedisClient{
		Prefix: "test",
	}
	key := rc.GetKey("mykey")
	assert.Equal(t, "test:mykey", key)
}

func TestRedis_GetClient(t *testing.T) {
	rc := &RedisClient{}
	client := rc.GetClient()
	assert.Nil(t, client)

	mr, client2 := setupMiniRedis(t)
	defer mr.Close()
	defer client2.Close()

	rc.Client = client2
	client = rc.GetClient()
	assert.NotNil(t, client)
}
