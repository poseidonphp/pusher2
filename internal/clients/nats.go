package clients

import (
	"crypto/tls"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NatsClient wraps a NATS connection and its JetStream context. It mirrors
// RedisClient's shape so loadAdapter/loadCacheManager can wire it up the
// same way regardless of which backend driver is selected.
type NatsClient struct {
	Conn   *nats.Conn
	JS     jetstream.JetStream
	Prefix string
}

// GetKey builds a prefixed key/subject token, mirroring RedisClient.GetKey.
func (n *NatsClient) GetKey(key string) string {
	return n.Prefix + "." + key
}

// InitNats connects to the given NATS URL and sets up a JetStream context
// (needed by the NATS-backed cache's KV store). useTls forces TLS even if
// the URL doesn't declare it, same as RedisClient.InitRedis's useTls param.
func (n *NatsClient) InitNats(natsUrl string, useTls bool) error {
	if n.Conn != nil {
		return nil
	}

	opts := []nats.Option{nats.Name("pusher")}
	if useTls {
		opts = append(opts, nats.Secure(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	conn, err := nats.Connect(natsUrl, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to nats at %s: %w", natsUrl, err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create jetstream context: %w", err)
	}

	n.Conn = conn
	n.JS = js
	return nil
}
