package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type durableHandler struct {
	typeOf   reflect.Type
	dispatch func(context.Context, json.RawMessage) error
}

var durableHandlers = struct {
	sync.RWMutex
	items map[string]durableHandler
}{items: make(map[string]durableHandler)}

// RegisterDurable declares the concrete payload type used by an outbox event.
// Re-registering the same event/type pair is harmless; conflicting types panic
// during startup instead of silently discarding events at runtime.
func RegisterDurable[T any](eventType string) {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		panic("eventbus: durable event type is required")
	}
	typeOf := reflect.TypeFor[T]()
	if typeOf == nil {
		panic("eventbus: durable event payload type is required")
	}
	entry := durableHandler{
		typeOf: typeOf,
		dispatch: func(ctx context.Context, payload json.RawMessage) error {
			var event T
			if err := json.Unmarshal(payload, &event); err != nil {
				return fmt.Errorf("eventbus: decode %s: %w", eventType, err)
			}
			return Publish(ctx, event)
		},
	}

	durableHandlers.Lock()
	defer durableHandlers.Unlock()
	if existing, ok := durableHandlers.items[eventType]; ok {
		if existing.typeOf != typeOf {
			panic(fmt.Sprintf("eventbus: durable event %s already registered as %s", eventType, existing.typeOf))
		}
		return
	}
	durableHandlers.items[eventType] = entry
}

func dispatchDurable(ctx context.Context, eventType string, payload json.RawMessage) (bool, error) {
	durableHandlers.RLock()
	entry, ok := durableHandlers.items[strings.TrimSpace(eventType)]
	durableHandlers.RUnlock()
	if !ok {
		return false, nil
	}
	return true, entry.dispatch(ctx, payload)
}

// DurableEvent is the transaction-safe input used to persist a domain event
// and its outbox delivery record atomically with the aggregate mutation.
type DurableEvent struct {
	TenantID       int64
	TraceID        string
	IdempotencyKey string
	SchemaVersion  int
	EventType      string
	Payload        any
	Source         string
	AggregateID    string
	ActorID        string
	ActorType      string
	CreatedAt      time.Time
	MaxRetries     int
	ReplayIfExists bool
}

// EnqueueTx writes the domain event and outbox row using the caller's
// transaction. The stable idempotency key makes retries safe.
func EnqueueTx(tx *gorm.DB, input DurableEvent) (bool, error) {
	if tx == nil {
		return false, errors.New("eventbus: transaction is required")
	}
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return false, errors.New("eventbus: event type is required")
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return false, fmt.Errorf("eventbus: marshal %s payload: %w", eventType, err)
	}
	if !json.Valid(payload) {
		return false, fmt.Errorf("eventbus: invalid %s payload", eventType)
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = "event:" + uuid.NewString()
	}
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	schemaVersion := input.SchemaVersion
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	event := &models.DomainEvent{
		TenantID:       input.TenantID,
		TraceID:        strings.TrimSpace(input.TraceID),
		IdempotencyKey: idempotencyKey,
		SchemaVersion:  schemaVersion,
		EventType:      eventType,
		Payload:        string(payload),
		Source:         strings.TrimSpace(input.Source),
		AggregateID:    strings.TrimSpace(input.AggregateID),
		ActorID:        strings.TrimSpace(input.ActorID),
		ActorType:      strings.TrimSpace(input.ActorType),
		CreatedAt:      createdAt,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(event)
	if result.Error != nil {
		return false, fmt.Errorf("eventbus: create domain event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if !input.ReplayIfExists {
			return false, nil
		}
		var existing models.DomainEvent
		if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error; err != nil {
			return false, fmt.Errorf("eventbus: load existing domain event: %w", err)
		}
		now := time.Now()
		requeued := tx.Model(&models.OutboxRecord{}).
			Where("event_id = ? AND (status IN ? OR (status = ? AND (locked_until IS NULL OR locked_until <= ?)))",
				existing.ID,
				[]string{models.OutboxStatusPublished, models.OutboxStatusFailed, models.OutboxStatusDead},
				models.OutboxStatusPublishing,
				now).
			Updates(map[string]any{
				"status":        models.OutboxStatusPending,
				"retry_count":   0,
				"last_error":    "",
				"next_retry_at": nil,
				"locked_until":  nil,
				"published_at":  nil,
			})
		if requeued.Error != nil {
			return false, fmt.Errorf("eventbus: requeue existing outbox record: %w", requeued.Error)
		}
		return requeued.RowsAffected == 1, nil
	}
	maxRetries := input.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}
	if err := tx.Create(&models.OutboxRecord{
		EventID:    event.ID,
		EventType:  eventType,
		Status:     models.OutboxStatusPending,
		MaxRetries: maxRetries,
		CreatedAt:  createdAt,
	}).Error; err != nil {
		return false, fmt.Errorf("eventbus: create outbox record: %w", err)
	}
	return true, nil
}
