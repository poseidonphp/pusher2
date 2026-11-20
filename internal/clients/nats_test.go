package clients

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNats_InitNats(t *testing.T) {
	_, _, url := setupTestNatsClient(t)

	nc := &NatsClient{Prefix: "test"}
	err := nc.InitNats(url, false)
	assert.NoError(t, err)
	assert.NotNil(t, nc.Conn)
	assert.NotNil(t, nc.JS)
	defer nc.Conn.Close()

	// Calling it again with an existing connection should be a no-op, not
	// open a second connection.
	err = nc.InitNats(url, false)
	assert.NoError(t, err)
}

func TestNats_InitNats_BadURL(t *testing.T) {
	nc := &NatsClient{}
	err := nc.InitNats("nats://localhost:4111", false) // assuming nothing is running on 4111
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to nats")
}

func TestNats_GetKey(t *testing.T) {
	nc := &NatsClient{Prefix: "test"}
	key := nc.GetKey("mykey")
	assert.Equal(t, "test.mykey", key)
}
