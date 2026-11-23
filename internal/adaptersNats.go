package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"pusher/internal/metrics"
	"pusher/log"
)

const (
	// natsPresenceInterval is how often a node announces itself on the
	// presence subject, and how often the stale-peer sweep runs.
	natsPresenceInterval = 2 * time.Second
	// natsPresenceTTL is how long a peer can go without a heartbeat before
	// it's considered gone.
	natsPresenceTTL = 3 * natsPresenceInterval
)

// NatsAdapter implements HorizontalInterface on top of a NATS connection.
//
// NATS has no server-side "how many subscribers on this subject" query
// (unlike Redis's PUBSUB NUMSUB), which HorizontalAdapter.getNumSub()
// normally relies on to skip the distributed round-trip when alone and to
// return early once every peer has replied. NatsAdapter replaces that with
// a lightweight peer-presence heartbeat: each node periodically announces
// itself on a presence subject, and getNumSub() reports the live count.
type NatsAdapter struct {
	*HorizontalAdapter
	ctx     context.Context
	Channel string // base subject, e.g. "pusher.int"
	conn    *nats.Conn
	sub     *nats.Subscription

	peersMu sync.Mutex
	peers   map[string]time.Time

	cancelBackground context.CancelFunc
}

// NewNatsAdapter creates a NatsAdapter and the HorizontalAdapter wrapping
// it, mirroring NewRedisAdapter's shape.
func NewNatsAdapter(ctx context.Context, conn *nats.Conn, prefix string, channel string, metricsManager metrics.MetricsInterface) (*NatsAdapter, error) {
	n := &NatsAdapter{
		ctx:     ctx,
		conn:    conn,
		Channel: channel,
		peers:   make(map[string]time.Time),
	}
	if prefix != "" {
		n.Channel = fmt.Sprintf("%s.%s", prefix, channel)
	}

	// Create the horizontal adapter, injecting this instance that satisfies the HorizontalInterface
	ha, err := NewHorizontalAdapter(n, metricsManager)
	if err != nil {
		log.Logger().Errorf("Error creating HorizontalAdapter: %v", err)
		return nil, err
	}
	n.HorizontalAdapter = ha

	// Initialize the NATS adapter
	err = n.Init()
	if err != nil {
		log.Logger().Errorf("Error initializing NATS adapter: %v", err)
		return nil, err
	}
	return n, nil
}

func (na *NatsAdapter) Init() error {
	if na.conn == nil {
		return fmt.Errorf("nats connection not initialized")
	}

	na.requestChannel = na.Channel + ".comms.req"
	na.responseChannel = na.Channel + ".comms.res"

	if err := na.subscribe(); err != nil {
		return err
	}

	backgroundCtx, cancel := context.WithCancel(na.ctx)
	na.cancelBackground = cancel
	go na.heartbeat(backgroundCtx)
	go na.evictStalePeers(backgroundCtx)

	return nil
}

func (na *NatsAdapter) GetChannelName() string {
	return na.Channel + ".msg"
}

func (na *NatsAdapter) presenceSubject() string {
	return na.Channel + ".presence"
}

func (na *NatsAdapter) broadcastToChannel(channel string, data string) {
	if err := na.conn.Publish(channel, []byte(data)); err != nil {
		log.Logger().Errorf("Error publishing to nats subject %s: %v", channel, err)
	}
}

// getNumSub returns the number of known live peers, including self,
// tracked via the presence heartbeat (see the NatsAdapter doc comment).
func (na *NatsAdapter) getNumSub() (int64, error) {
	na.peersMu.Lock()
	defer na.peersMu.Unlock()
	return int64(len(na.peers)) + 1, nil
}

// subscribe sets up the single wildcard subscription that catches every
// subject under this adapter's base channel (messages, requests,
// responses, and presence); onMessage dispatches by exact subject.
func (na *NatsAdapter) subscribe() error {
	sub, err := na.conn.Subscribe(na.Channel+".>", na.onMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s.>: %w", na.Channel, err)
	}
	na.sub = sub
	log.Logger().Trace("subscribed to nats subject: ", na.Channel+".>")
	return nil
}

func (na *NatsAdapter) onMessage(msg *nats.Msg) {
	switch msg.Subject {
	case na.requestChannel:
		log.Logger().Tracef(" ... Step 1 calling onRequest()")
		dataToSend, err := na.onRequest(msg.Subject, string(msg.Data))
		log.Logger().Tracef(" ... Step 2 got response from onRequest(): %s", dataToSend)
		if err != nil {
			log.Logger().Errorf("Error processing request: %v", err)
			return
		}
		if dataToSend != nil {
			na.broadcastToChannel(na.responseChannel, string(dataToSend))
		}
	case na.responseChannel:
		na.onResponse(msg.Subject, string(msg.Data))
	case na.presenceSubject():
		na.onPresence(string(msg.Data))
	case na.GetChannelName():
		na.onBroadcast(string(msg.Data))
	default:
		log.Logger().Tracef("Received message on unexpected subject %s", msg.Subject)
	}
}

// onBroadcast handles cross-node message delivery: used for other nodes
// sending messages to local sockets. Equivalent of the Redis adapter's
// onMessage handler for its psubscribe pattern.
func (na *NatsAdapter) onBroadcast(message string) {
	var decodedMessage *horizontalPubsubBroadcastedMessage
	if err := json.Unmarshal([]byte(message), &decodedMessage); err != nil {
		log.Logger().Errorf("Could not unmarshal message: %v", err)
		return
	}

	if na.uuid == decodedMessage.Uuid || decodedMessage.AppID == "" || decodedMessage.Channel == "" || decodedMessage.Data == "" {
		return
	}

	data, ok := decodedMessage.Data.(string)
	if !ok {
		return
	}

	na.sendLocally(decodedMessage.AppID, decodedMessage.Channel, data, decodedMessage.ExceptingIds)
}

func (na *NatsAdapter) onPresence(peerUuid string) {
	if peerUuid == "" || peerUuid == na.uuid {
		return
	}
	na.peersMu.Lock()
	na.peers[peerUuid] = time.Now()
	na.peersMu.Unlock()
}

func (na *NatsAdapter) heartbeat(ctx context.Context) {
	announce := func() {
		if err := na.conn.Publish(na.presenceSubject(), []byte(na.uuid)); err != nil {
			log.Logger().Errorf("Error publishing presence: %v", err)
		}
	}
	// Announce immediately so peers discover a new node within one tick
	// instead of waiting a full interval.
	announce()

	ticker := time.NewTicker(natsPresenceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			announce()
		}
	}
}

func (na *NatsAdapter) evictStalePeers(ctx context.Context) {
	ticker := time.NewTicker(natsPresenceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-natsPresenceTTL)
			na.peersMu.Lock()
			for peerUuid, lastSeen := range na.peers {
				if lastSeen.Before(cutoff) {
					delete(na.peers, peerUuid)
				}
			}
			na.peersMu.Unlock()
		}
	}
}

func (na *NatsAdapter) disconnect() {
	if na.cancelBackground != nil {
		na.cancelBackground()
	}
	if na.sub != nil {
		_ = na.sub.Unsubscribe()
	}
}
