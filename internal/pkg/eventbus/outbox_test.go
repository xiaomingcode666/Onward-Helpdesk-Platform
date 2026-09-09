package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type durableOutboxTestEvent struct {
	Value string `json:"value"`
}

func TestOutboxPublishMarksImmediateDeliveryAndDoesNotRepublish(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatalf("migrate outbox tables: %v", err)
	}
	sqls.SetDB(db)

	bus := New[any]()
	var deliveries atomic.Int32
	bus.Subscribe(func(_ context.Context, _ any) error {
		deliveries.Add(1)
		return nil
	})
	publisher := NewOutboxPublisher(bus)
	if err := publisher.Publish(context.Background(), models.DomainEvent{
		IdempotencyKey: "outbox-immediate-once",
		EventType:      "test.outbox",
		Payload:        `{"value":"ok"}`,
	}); err != nil {
		t.Fatalf("publish outbox event: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var record models.OutboxRecord
		if err := db.First(&record).Error; err == nil && record.Status == models.OutboxStatusPublished && deliveries.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if deliveries.Load() != 1 {
		t.Fatalf("expected one immediate delivery, got %d", deliveries.Load())
	}
	var record models.OutboxRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("load outbox record: %v", err)
	}
	if record.Status != models.OutboxStatusPublished || record.PublishedAt == nil || record.LockedUntil != nil {
		t.Fatalf("unexpected delivered outbox record: %#v", record)
	}

	publisher.processPendingOutbox(context.Background())
	if deliveries.Load() != 1 {
		t.Fatalf("published record was delivered again, count=%d", deliveries.Load())
	}
}

func TestOutboxPublishWithIdempotencyKeyCreatesOneEvent(t *testing.T) {
	db := setupOutboxTestDB(t)
	bus := New[any]()
	var deliveries atomic.Int32
	bus.Subscribe(func(_ context.Context, _ any) error {
		deliveries.Add(1)
		return nil
	})
	publisher := NewOutboxPublisher(bus)
	event := models.DomainEvent{
		IdempotencyKey: "ticket:42:closed",
		EventType:      "ticket.closed",
		Payload:        `{"ticketId":42}`,
	}
	if err := publisher.PublishWithIdempotencyKey(context.Background(), event); err != nil {
		t.Fatalf("first idempotent publish: %v", err)
	}
	waitForOutboxDeliveries(t, deliveries.Load, 1)
	if err := publisher.PublishWithIdempotencyKey(context.Background(), event); err != nil {
		t.Fatalf("duplicate idempotent publish: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	var eventCount int64
	var outboxCount int64
	if err := db.Model(&models.DomainEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count domain events: %v", err)
	}
	if err := db.Model(&models.OutboxRecord{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox records: %v", err)
	}
	if eventCount != 1 || outboxCount != 1 || deliveries.Load() != 1 {
		t.Fatalf("duplicate publish was not idempotent: events=%d outbox=%d deliveries=%d", eventCount, outboxCount, deliveries.Load())
	}
}

func TestOutboxPublishWithoutCallerKeyCreatesIndependentEvents(t *testing.T) {
	db := setupOutboxTestDB(t)
	bus := New[any]()
	var deliveries atomic.Int32
	bus.Subscribe(func(_ context.Context, _ any) error {
		deliveries.Add(1)
		return nil
	})
	publisher := NewOutboxPublisher(bus)
	for i := 0; i < 2; i++ {
		if err := publisher.Publish(context.Background(), models.DomainEvent{
			EventType: "test.unkeyed",
			Payload:   `{"ok":true}`,
		}); err != nil {
			t.Fatalf("publish unkeyed event %d: %v", i, err)
		}
	}
	waitForOutboxDeliveries(t, deliveries.Load, 2)

	var eventCount int64
	var distinctKeys int64
	if err := db.Model(&models.DomainEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count domain events: %v", err)
	}
	if err := db.Model(&models.DomainEvent{}).Distinct("idempotency_key").Count(&distinctKeys).Error; err != nil {
		t.Fatalf("count idempotency keys: %v", err)
	}
	if eventCount != 2 || distinctKeys != 2 {
		t.Fatalf("unkeyed events collided: events=%d distinct_keys=%d", eventCount, distinctKeys)
	}
}

func TestOutboxReplaysRegisteredConcreteEventType(t *testing.T) {
	db := setupOutboxTestDB(t)
	eventType := "test.outbox.typed." + t.Name()
	RegisterDurable[durableOutboxTestEvent](eventType)
	var received atomic.Int32
	bus := Get[durableOutboxTestEvent]()
	_, unsubscribe := bus.Subscribe(func(_ context.Context, event durableOutboxTestEvent) error {
		if event.Value != "replayed" {
			return errors.New("unexpected typed payload")
		}
		received.Add(1)
		return nil
	})
	defer unsubscribe()

	created, err := EnqueueTx(db, DurableEvent{
		TenantID:       1,
		IdempotencyKey: "typed-replay:" + t.Name(),
		EventType:      eventType,
		Payload:        durableOutboxTestEvent{Value: "replayed"},
		Source:         "test",
	})
	if err != nil || !created {
		t.Fatalf("enqueue typed event: created=%v err=%v", created, err)
	}

	publisher := NewOutboxPublisher(New[any]())
	publisher.processPendingOutbox(context.Background())
	if received.Load() != 1 {
		t.Fatalf("typed deliveries = %d, want 1", received.Load())
	}
	var record models.OutboxRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("load outbox record: %v", err)
	}
	if record.Status != models.OutboxStatusPublished {
		t.Fatalf("outbox status = %q, want published", record.Status)
	}
}

func TestEnqueueTxRollsBackDomainEventAndOutboxTogether(t *testing.T) {
	db := setupOutboxTestDB(t)
	errRollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		created, enqueueErr := EnqueueTx(tx, DurableEvent{
			TenantID:       1,
			IdempotencyKey: "rollback:" + t.Name(),
			EventType:      "test.rollback",
			Payload:        map[string]any{"ok": true},
		})
		if enqueueErr != nil || !created {
			t.Fatalf("enqueue rollback event: created=%v err=%v", created, enqueueErr)
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction error = %v", err)
	}
	var eventCount, outboxCount int64
	db.Model(&models.DomainEvent{}).Count(&eventCount)
	db.Model(&models.OutboxRecord{}).Count(&outboxCount)
	if eventCount != 0 || outboxCount != 0 {
		t.Fatalf("rollback leaked rows: events=%d outbox=%d", eventCount, outboxCount)
	}
}

func TestEnqueueTxCanRequeuePublishedEventForExplicitRecovery(t *testing.T) {
	db := setupOutboxTestDB(t)
	input := DurableEvent{
		TenantID:       1,
		IdempotencyKey: "requeue:" + t.Name(),
		EventType:      "test.requeue",
		Payload:        map[string]any{"value": "stable"},
	}
	created, err := EnqueueTx(db, input)
	if err != nil || !created {
		t.Fatalf("enqueue event: created=%v err=%v", created, err)
	}
	var record models.OutboxRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	now := time.Now()
	if err := db.Model(&models.OutboxRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status":       models.OutboxStatusPublished,
		"published_at": &now,
	}).Error; err != nil {
		t.Fatalf("mark published: %v", err)
	}
	input.ReplayIfExists = true
	requeued, err := EnqueueTx(db, input)
	if err != nil || !requeued {
		t.Fatalf("requeue event: requeued=%v err=%v", requeued, err)
	}
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if record.Status != models.OutboxStatusPending || record.PublishedAt != nil || record.RetryCount != 0 {
		t.Fatalf("unexpected requeued record: %#v", record)
	}
}

func TestOutboxMovesExhaustedDeliveryToDeadLetterAndCanRequeue(t *testing.T) {
	db := setupOutboxTestDB(t)
	eventType := "test.outbox.dead." + t.Name()
	RegisterDurable[durableOutboxTestEvent](eventType)
	var attempts atomic.Int32
	bus := Get[durableOutboxTestEvent]()
	_, unsubscribe := bus.Subscribe(func(_ context.Context, _ durableOutboxTestEvent) error {
		attempts.Add(1)
		return errors.New("consumer unavailable")
	})
	defer unsubscribe()

	input := DurableEvent{
		TenantID:       1,
		IdempotencyKey: "dead-letter:" + t.Name(),
		EventType:      eventType,
		Payload:        durableOutboxTestEvent{Value: "retry-me"},
		MaxRetries:     2,
	}
	created, err := EnqueueTx(db, input)
	if err != nil || !created {
		t.Fatalf("enqueue event: created=%v err=%v", created, err)
	}
	publisher := NewOutboxPublisher(New[any]())
	publisher.processPendingOutbox(context.Background())
	if err := db.Model(&models.OutboxRecord{}).Where("1 = 1").Update("next_retry_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("make retry due: %v", err)
	}
	publisher.processPendingOutbox(context.Background())

	var record models.OutboxRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("load outbox record: %v", err)
	}
	if record.Status != models.OutboxStatusDead || record.RetryCount != 2 || record.NextRetryAt != nil || record.LockedUntil != nil {
		t.Fatalf("unexpected dead letter state: %#v", record)
	}
	publisher.processPendingOutbox(context.Background())
	if attempts.Load() != 2 {
		t.Fatalf("dead letter was delivered again: attempts=%d", attempts.Load())
	}

	input.ReplayIfExists = true
	requeued, err := EnqueueTx(db, input)
	if err != nil || !requeued {
		t.Fatalf("requeue dead letter: requeued=%v err=%v", requeued, err)
	}
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("reload outbox record: %v", err)
	}
	if record.Status != models.OutboxStatusPending || record.RetryCount != 0 || record.LastError != "" {
		t.Fatalf("unexpected requeued dead letter: %#v", record)
	}
}

func setupOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatalf("migrate outbox tables: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func waitForOutboxDeliveries(t *testing.T, current func() int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if current() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("outbox deliveries = %d, want %d", current(), want)
}
