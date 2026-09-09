package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	startDefaultOutboxPublisher sync.Once
	defaultOutboxPublisherMu    sync.RWMutex
	defaultOutboxPublisher      *OutboxPublisher
)

// StartDefaultOutboxPublisher starts the process-wide durable outbox relay.
// Registered durable events are decoded into their concrete Go type. Unknown
// event types retain the legacy Bus[any] delivery behavior.
func StartDefaultOutboxPublisher(ctx context.Context) {
	startDefaultOutboxPublisher.Do(func() {
		publisher := NewOutboxPublisher(Get[any]())
		defaultOutboxPublisherMu.Lock()
		defaultOutboxPublisher = publisher
		defaultOutboxPublisherMu.Unlock()
		go publisher.Start(ctx)
	})
}

// WakeDefaultOutboxPublisher asks the process-wide relay to poll immediately.
// The periodic poll remains the crash-recovery fallback.
func WakeDefaultOutboxPublisher() {
	defaultOutboxPublisherMu.RLock()
	publisher := defaultOutboxPublisher
	defaultOutboxPublisherMu.RUnlock()
	if publisher != nil {
		publisher.Wake()
	}
}

// OutboxPublisher 事件 Outbox 发布器。
// 事件先写入 domain_events 表，再由异步协程轮询投递。
type OutboxPublisher struct {
	bus          *Bus[any]
	poolInterval time.Duration
	stopCh       chan struct{}
	wakeCh       chan struct{}
}

// OutboxPublisherOption 配置 OutboxPublisher 的选项
type OutboxPublisherOption func(*OutboxPublisher)

// WithPoolInterval 设置轮询间隔
func WithPoolInterval(d time.Duration) OutboxPublisherOption {
	return func(op *OutboxPublisher) {
		if d > 0 {
			op.poolInterval = d
		}
	}
}

// NewOutboxPublisher 创建 OutboxPublisher 实例
func NewOutboxPublisher(bus *Bus[any], opts ...OutboxPublisherOption) *OutboxPublisher {
	op := &OutboxPublisher{
		bus:          bus,
		poolInterval: 5 * time.Second,
		stopCh:       make(chan struct{}),
		wakeCh:       make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(op)
	}
	return op
}

// Publish 将事件写入 domain_events 表和 outbox_records 表，再异步发布
func (op *OutboxPublisher) Publish(ctx context.Context, event models.DomainEvent) error {
	if event.IdempotencyKey == "" {
		event.IdempotencyKey = "event:" + uuid.NewString()
	}
	outbox, created, err := op.persist(event)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	// Reuse the leased relay path for immediate delivery so a successful
	// publish is also durably marked and will not be published again by polling.
	go op.processSingleRecord(context.WithoutCancel(ctx), outbox)

	return nil
}

// PublishWithIdempotencyKey 使用幂等键发布事件（幂等键相同则跳过）
func (op *OutboxPublisher) PublishWithIdempotencyKey(ctx context.Context, event models.DomainEvent) error {
	if event.IdempotencyKey == "" {
		return op.Publish(ctx, event)
	}
	return op.Publish(ctx, event)
}

func (op *OutboxPublisher) persist(event models.DomainEvent) (*models.OutboxRecord, bool, error) {
	event.CreatedAt = time.Now()
	var outbox *models.OutboxRecord
	created := false
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(&event)
		if result.Error != nil {
			return fmt.Errorf("outbox: create domain_event failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		created = true
		outbox = &models.OutboxRecord{
			EventID:    event.ID,
			EventType:  event.EventType,
			Status:     models.OutboxStatusPending,
			MaxRetries: 3,
			CreatedAt:  time.Now(),
		}
		if err := tx.Create(outbox).Error; err != nil {
			return fmt.Errorf("outbox: create outbox_record failed: %w", err)
		}
		return nil
	})
	return outbox, created, err
}

// Start 启动后台轮询，定时处理失败的 Outbox 记录
func (op *OutboxPublisher) Start(ctx context.Context) {
	ticker := time.NewTicker(op.poolInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			op.processPendingOutbox(ctx)
		case <-op.wakeCh:
			op.processPendingOutbox(ctx)
		case <-op.stopCh:
			slog.Info("outbox publisher stopped")
			return
		case <-ctx.Done():
			return
		}
	}
}

// Wake triggers a non-blocking immediate poll.
func (op *OutboxPublisher) Wake() {
	select {
	case op.wakeCh <- struct{}{}:
	default:
	}
}

// ProcessPending runs one relay pass. It is useful for deterministic recovery
// jobs and tests; normal application code should use Wake.
func (op *OutboxPublisher) ProcessPending(ctx context.Context) {
	op.processPendingOutbox(ctx)
}

// Stop 停止后台轮询
func (op *OutboxPublisher) Stop() {
	close(op.stopCh)
}

// processPendingOutbox 处理待发布的 Outbox 记录
func (op *OutboxPublisher) processPendingOutbox(ctx context.Context) {
	var pendingRecords []models.OutboxRecord
	now := time.Now()
	sqls.DB().Where("retry_count < max_retries AND ((status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND (locked_until IS NULL OR locked_until <= ?)))",
		[]string{models.OutboxStatusPending, models.OutboxStatusFailed}, now,
		models.OutboxStatusPublishing, now).
		Limit(100).
		Find(&pendingRecords)

	for _, record := range pendingRecords {
		op.processSingleRecord(ctx, &record)
	}
}

// processSingleRecord 处理单条 Outbox 记录
func (op *OutboxPublisher) processSingleRecord(ctx context.Context, record *models.OutboxRecord) {
	// Claim with a lease so multiple application instances cannot publish the
	// same record concurrently and a crashed publisher can be recovered.
	now := time.Now()
	lockedUntil := now.Add(time.Minute)
	claim := sqls.DB().Model(&models.OutboxRecord{}).
		Where("id = ? AND retry_count < max_retries AND ((status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND (locked_until IS NULL OR locked_until <= ?)))",
			record.ID, []string{models.OutboxStatusPending, models.OutboxStatusFailed}, now,
			models.OutboxStatusPublishing, now).
		Updates(map[string]any{
			"status":       models.OutboxStatusPublishing,
			"locked_until": lockedUntil,
		})
	if claim.Error != nil || claim.RowsAffected != 1 {
		return
	}

	// 查询原始事件
	var event models.DomainEvent
	if err := sqls.DB().First(&event, record.EventID).Error; err != nil {
		slog.Error("outbox: domain event not found", "eventID", record.EventID, "error", err)
		op.failRecord(record, now, err)
		return
	}

	now = time.Now()
	payloadJSON := json.RawMessage(event.Payload)
	if len(payloadJSON) == 0 {
		payloadJSON = json.RawMessage("null")
	}
	if !json.Valid(payloadJSON) {
		op.failRecord(record, now, fmt.Errorf("outbox: invalid JSON payload for %s", event.EventType))
		return
	}
	dispatched, err := dispatchDurable(ctx, event.EventType, payloadJSON)
	if err == nil && !dispatched {
		var payload any
		if decodeErr := json.Unmarshal(payloadJSON, &payload); decodeErr != nil {
			err = decodeErr
		} else {
			err = op.bus.Publish(ctx, payload)
		}
	}
	if err != nil {
		slog.Error("outbox: publish to bus failed", "eventID", record.EventID, "error", err)
		op.failRecord(record, now, err)
		return
	}

	// 更新事件发布时间
	sqls.DB().Model(&models.DomainEvent{}).Where("id = ?", event.ID).
		Update("published_at", now)

	// 标记 outbox 为已发布
	record.Status = models.OutboxStatusPublished
	record.PublishedAt = &now
	record.LockedUntil = nil
	if err := sqls.DB().Save(record).Error; err != nil {
		slog.Error("outbox: mark delivery published failed", "outboxID", record.ID, "error", err)
	}
}

func (op *OutboxPublisher) failRecord(record *models.OutboxRecord, now time.Time, err error) {
	record.RetryCount++
	record.LastError = err.Error()
	record.LockedUntil = nil
	record.NextRetryAt = nil
	if record.RetryCount >= record.MaxRetries {
		record.Status = models.OutboxStatusDead
		slog.Error("outbox: delivery moved to dead letter", "outboxID", record.ID, "eventID", record.EventID, "eventType", record.EventType, "retryCount", record.RetryCount)
	} else {
		record.Status = models.OutboxStatusPending
		nextRetry := now.Add(time.Duration(record.RetryCount*record.RetryCount) * time.Minute)
		record.NextRetryAt = &nextRetry
	}
	if saveErr := sqls.DB().Save(record).Error; saveErr != nil {
		slog.Error("outbox: save failed delivery state", "outboxID", record.ID, "error", saveErr)
	}
}
