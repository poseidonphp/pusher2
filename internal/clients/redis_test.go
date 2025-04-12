package clients

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis"
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
	_, err = client.Ping().Result()
	if err != nil {
		t.Fatalf("Could not connect to miniredis: %v", err)
	}

	return mr, client
}

func TestRedis_InitRedis(t *testing.T) {
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

	err = rc.InitRedis(add1, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, rc.Client)

}
