package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestNotificationQueueConsumesAndAcknowledgesJob(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	queue := newNotificationQueueService(client, config.RedisConfig{
		NotificationStream:        "test:notification:consume",
		NotificationConsumerGroup: "test-notification-workers",
	})
	queue.blockTimeout = 10 * time.Millisecond
	queue.reclaimInterval = time.Hour

	received := make(chan string, 1)
	queue.Register("test.notification", func(ctx context.Context, payload json.RawMessage) error {
		var value string
		if err := json.Unmarshal(payload, &value); err != nil {
			return err
		}
		received <- value
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	if err := queue.Start(ctx); err != nil {
		t.Fatalf("start notification queue: %v", err)
	}
	if _, err := queue.Enqueue(ctx, "test.notification", "queued"); err != nil {
		t.Fatalf("enqueue notification: %v", err)
	}
	select {
	case got := <-received:
		if got != "queued" {
			t.Fatalf("payload = %q, want queued", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification job was not consumed")
	}
	waitForQueueCondition(t, func() bool {
		pending, err := client.XPending(context.Background(), queue.stream, queue.group).Result()
		return err == nil && pending.Count == 0
	})
	cancel()
	queue.Wait()
}

func TestNotificationQueueMovesExhaustedJobToDeadLetter(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	queue := newNotificationQueueService(client, config.RedisConfig{
		NotificationStream:        "test:notification:retry",
		NotificationConsumerGroup: "test-notification-workers",
	})
	queue.maxDeliveries = 2
	queue.Register("test.failure", func(context.Context, json.RawMessage) error {
		return errors.New("delivery failed")
	})
	ctx := context.Background()
	if err := queue.ensureConsumerGroup(ctx); err != nil {
		t.Fatalf("create consumer group: %v", err)
	}
	if _, err := queue.Enqueue(ctx, "test.failure", map[string]string{"value": "retry"}); err != nil {
		t.Fatalf("enqueue failure job: %v", err)
	}
	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    queue.group,
		Consumer: queue.consumer,
		Streams:  []string{queue.stream, ">"},
		Count:    1,
	}).Result()
	if err != nil || len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("read queued job: streams=%v err=%v", streams, err)
	}
	message := streams[0].Messages[0]
	queue.processMessage(ctx, message)
	claimed, err := client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   queue.stream,
		Group:    queue.group,
		Consumer: queue.consumer,
		MinIdle:  0,
		Messages: []string{message.ID},
	}).Result()
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim failed job: messages=%v err=%v", claimed, err)
	}
	queue.processMessage(ctx, claimed[0])

	pending, err := client.XPending(ctx, queue.stream, queue.group).Result()
	if err != nil {
		t.Fatalf("read pending jobs: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count)
	}
	if got := client.XLen(ctx, queue.deadLetterStream).Val(); got != 1 {
		t.Fatalf("dead letter count = %d, want 1", got)
	}
}

func TestNotificationQueueTreatsInactiveRecipientAsNonRetryable(t *testing.T) {
	if !isNonRetryableNotificationCreateError(errorsx.Forbidden("notification recipient is outside the tenant or inactive")) {
		t.Fatal("inactive notification recipient error should be non-retryable")
	}
	if isNonRetryableNotificationCreateError(errors.New("notification recipient is outside the tenant or inactive")) {
		t.Fatal("plain errors should remain retryable")
	}
	if isNonRetryableNotificationCreateError(errorsx.Forbidden("notification ticket is outside the tenant")) {
		t.Fatal("resource scope errors should remain retryable")
	}
}

func TestNotificationQueueReclaimsAbandonedJob(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	queue := newNotificationQueueService(client, config.RedisConfig{
		NotificationStream:        "test:notification:reclaim",
		NotificationConsumerGroup: "test-notification-workers",
	})
	queue.reclaimIdle = 0

	received := make(chan string, 1)
	queue.Register("test.reclaim", func(ctx context.Context, payload json.RawMessage) error {
		var value string
		if err := json.Unmarshal(payload, &value); err != nil {
			return err
		}
		received <- value
		return nil
	})
	ctx := context.Background()
	if err := queue.ensureConsumerGroup(ctx); err != nil {
		t.Fatalf("create consumer group: %v", err)
	}
	if _, err := queue.Enqueue(ctx, "test.reclaim", "abandoned"); err != nil {
		t.Fatalf("enqueue notification job: %v", err)
	}
	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    queue.group,
		Consumer: "stopped-worker",
		Streams:  []string{queue.stream, ">"},
		Count:    1,
	}).Result()
	if err != nil || len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("read abandoned job: streams=%v err=%v", streams, err)
	}
	if err := queue.reclaimPending(ctx); err != nil {
		t.Fatalf("reclaim abandoned job: %v", err)
	}
	select {
	case got := <-received:
		if got != "abandoned" {
			t.Fatalf("payload = %q, want abandoned", got)
		}
	default:
		t.Fatal("abandoned notification job was not processed")
	}
	pending, err := client.XPending(ctx, queue.stream, queue.group).Result()
	if err != nil {
		t.Fatalf("read pending jobs: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count)
	}
}

func waitForQueueCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queue condition was not satisfied")
}
