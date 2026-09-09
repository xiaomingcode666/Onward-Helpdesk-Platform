package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	realtimeBrokerChannel      = "remotehelpdesk:realtime:v1"
	realtimeBrokerKindEvent    = "event"
	realtimeBrokerKindProbe    = "presence_probe"
	realtimeBrokerOutboundSize = 512
	realtimePresenceTTL        = 75 * time.Second
	realtimePresenceKeyTTL     = 2 * realtimePresenceTTL
)

var removeRealtimePresenceScript = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[2])
local count = redis.call('ZCARD', KEYS[1])
if count == 0 then
  redis.call('DEL', KEYS[1])
else
  redis.call('EXPIRE', KEYS[1], ARGV[3])
end
return count
`)

type realtimeBrokerEnvelope struct {
	Kind           string          `json:"kind"`
	Source         string          `json:"source"`
	Target         string          `json:"target,omitempty"`
	Topics         []string        `json:"topics,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	ConversationID int64           `json:"conversationId,omitempty"`
}

type wsRealtimeBroker struct {
	instanceID string
	outbound   chan realtimeBrokerEnvelope
	client     *redis.Client
	started    atomic.Bool
	startOnce  sync.Once
	handler    func(realtimeBrokerEnvelope)
}

func newWSRealtimeBroker() *wsRealtimeBroker {
	return &wsRealtimeBroker{
		instanceID: uuid.NewString(),
		outbound:   make(chan realtimeBrokerEnvelope, realtimeBrokerOutboundSize),
	}
}

func (b *wsRealtimeBroker) Start(ctx context.Context, redisConfig config.RedisConfig, handler func(realtimeBrokerEnvelope)) {
	if b == nil || handler == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	b.startOnce.Do(func() {
		addr := strings.TrimSpace(redisConfig.Addr)
		if addr == "" {
			addr = ":6379"
		}
		b.client = redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     redisConfig.Password,
			DB:           redisConfig.DB,
			MaxRetries:   1,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		})
		b.handler = handler
		b.started.Store(true)
		go b.publishLoop(ctx)
		go b.subscribeLoop(ctx)
	})
}

func (b *wsRealtimeBroker) PublishEvent(topics []string, payload []byte, target string) {
	if b == nil || !b.started.Load() || len(topics) == 0 || len(payload) == 0 {
		return
	}
	b.enqueue(realtimeBrokerEnvelope{
		Kind:    realtimeBrokerKindEvent,
		Source:  b.instanceID,
		Target:  strings.TrimSpace(target),
		Topics:  append([]string(nil), topics...),
		Payload: append(json.RawMessage(nil), payload...),
	})
}

func (b *wsRealtimeBroker) ProbePresence(conversationID int64) {
	if b == nil || !b.started.Load() || conversationID <= 0 {
		return
	}
	b.enqueue(realtimeBrokerEnvelope{
		Kind:           realtimeBrokerKindProbe,
		Source:         b.instanceID,
		ConversationID: conversationID,
	})
}

func (b *wsRealtimeBroker) UpsertPresence(conversationID int64, actorID, sessionID string) bool {
	if !b.presenceRegistryAvailable(conversationID, actorID, sessionID) {
		return false
	}
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	key := realtimePresenceKey(conversationID, actorID)
	member := b.presenceMember(sessionID)
	_, err := b.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now.Add(realtimePresenceTTL).UnixMilli()), Member: member})
		pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprint(now.UnixMilli()))
		pipe.Expire(ctx, key, realtimePresenceKeyTTL)
		return nil
	})
	if err != nil {
		slog.Warn("upsert distributed realtime presence failed", "conversation_id", conversationID, "error", err)
		return false
	}
	return true
}

// RemovePresence returns whether another live session for the actor remains and
// whether Redis was available for an authoritative distributed check.
func (b *wsRealtimeBroker) RemovePresence(conversationID int64, actorID, sessionID string) (bool, bool) {
	if !b.presenceRegistryAvailable(conversationID, actorID, sessionID) {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	count, err := removeRealtimePresenceScript.Run(
		ctx,
		b.client,
		[]string{realtimePresenceKey(conversationID, actorID)},
		b.presenceMember(sessionID),
		time.Now().UnixMilli(),
		int64(realtimePresenceKeyTTL/time.Second),
	).Int64()
	if err != nil {
		slog.Warn("remove distributed realtime presence failed", "conversation_id", conversationID, "error", err)
		return false, false
	}
	return count > 0, true
}

func (b *wsRealtimeBroker) presenceRegistryAvailable(conversationID int64, actorID, sessionID string) bool {
	return b != nil && b.started.Load() && b.client != nil && conversationID > 0 &&
		strings.TrimSpace(actorID) != "" && strings.TrimSpace(sessionID) != ""
}

func (b *wsRealtimeBroker) presenceMember(sessionID string) string {
	return b.instanceID + ":" + strings.TrimSpace(sessionID)
}

func realtimePresenceKey(conversationID int64, actorID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(actorID)))
	return fmt.Sprintf("remotehelpdesk:realtime:presence:v1:%d:%x", conversationID, digest[:12])
}

func (b *wsRealtimeBroker) enqueue(envelope realtimeBrokerEnvelope) {
	select {
	case b.outbound <- envelope:
	default:
		slog.Warn("realtime broker outbound buffer full", "kind", envelope.Kind, "topic_count", len(envelope.Topics))
	}
}

func (b *wsRealtimeBroker) publishLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-b.outbound:
			payload, err := json.Marshal(envelope)
			if err != nil {
				continue
			}
			publishCtx, cancel := context.WithTimeout(ctx, time.Second)
			err = b.client.Publish(publishCtx, realtimeBrokerChannel, payload).Err()
			cancel()
			if err != nil {
				slog.Warn("publish realtime broker event failed", "kind", envelope.Kind, "error", err)
			}
		}
	}
}

func (b *wsRealtimeBroker) subscribeLoop(ctx context.Context) {
	pubsub := b.client.Subscribe(ctx, realtimeBrokerChannel)
	defer func() { _ = pubsub.Close() }()
	channel := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				return
			}
			var envelope realtimeBrokerEnvelope
			if err := json.Unmarshal([]byte(message.Payload), &envelope); err != nil {
				slog.Warn("decode realtime broker event failed", "error", err)
				continue
			}
			if envelope.Source == b.instanceID || (envelope.Target != "" && envelope.Target != b.instanceID) {
				continue
			}
			b.handler(envelope)
		}
	}
}
