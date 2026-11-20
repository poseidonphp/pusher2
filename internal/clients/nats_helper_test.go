package clients

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// setupTestNatsServer starts an embedded NATS server with JetStream
// enabled (needed for the KV-backed cache) on a random free port, the same
// role setupMiniRedis plays for Redis tests. The server and any connection
// made against it are cleaned up automatically when the test ends.
func setupTestNatsServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      server.RANDOM_PORT,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("could not create nats server: %v", err)
	}

	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatalf("nats server did not become ready in time")
	}
	t.Cleanup(ns.Shutdown)

	return ns
}

// setupTestNatsClient connects to a freshly started embedded NATS server
// and returns the connection, its JetStream context, and the server's
// client URL.
func setupTestNatsClient(t *testing.T) (*nats.Conn, jetstream.JetStream, string) {
	t.Helper()

	ns := setupTestNatsServer(t)
	url := ns.ClientURL()

	conn, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("could not connect to test nats server: %v", err)
	}
	t.Cleanup(conn.Close)

	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("could not create jetstream context: %v", err)
	}

	return conn, js, url
}
