package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
