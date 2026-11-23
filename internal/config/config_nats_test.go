package config

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestNatsServer starts an embedded NATS server with JetStream
// enabled on a random free port, the config package's own copy of the
// helper used by internal/clients and internal/cache (test helpers aren't
// exported across packages), playing the same role setupMiniRedis plays
// for the Redis driver-selection tests below.
func setupTestNatsServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      server.RANDOM_PORT,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second), "nats server did not become ready in time")
	t.Cleanup(ns.Shutdown)

	return ns
}

func TestInitializeServerConfig_NatsConfiguration(t *testing.T) {
	ns := setupTestNatsServer(t)

	testCases := []struct {
		name          string
		adapterDriver string
		cacheDriver   string
		expectedNats  bool
	}{
		{"All local drivers", "local", "local", false},
		{"Nats adapter only", "nats", "local", true},
		{"Nats cache only", "local", "nats", true},
		{"Both nats drivers", "nats", "nats", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupTestEnv(t)

			t.Setenv("ADAPTER_DRIVER", tc.adapterDriver)
			t.Setenv("CACHE_DRIVER", tc.cacheDriver)
			t.Setenv("NATS_URL", ns.ClientURL())

			ctx := context.Background()
			cfg, err := InitializeServerConfig(&ctx)

			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tc.expectedNats, cfg.UsingNats,
				"UsingNats should be %v for adapter=%s, cache=%s",
				tc.expectedNats, tc.adapterDriver, tc.cacheDriver)

			if tc.expectedNats {
				assert.NotNil(t, cfg.NatsInstance)
			} else {
				assert.Nil(t, cfg.NatsInstance)
			}
		})
	}
}

func TestInitializeServerConfig_NatsConfiguration_BadHost(t *testing.T) {
	setupTestEnv(t)

	t.Setenv("ADAPTER_DRIVER", "nats")
	t.Setenv("NATS_URL", "nats://baddaddress:9876")

	ctx := context.Background()
	cfg, err := InitializeServerConfig(&ctx)

	require.Error(t, err)
	require.Nil(t, cfg)
	assert.Contains(t, err.Error(), "Error initializing NATS client")
}
