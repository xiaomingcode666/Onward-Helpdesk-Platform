package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/utils"
)

// 事件类型常量
const (
	EventTicketCreated             = "ticket.created"
	EventTicketAssigned            = "ticket.assigned"
	EventTicketClosed              = "ticket.closed"
	EventDiagnosisCompleted        = "diagnosis.completed"
	EventDiagnosisHandoffDecided   = "diagnosis.handoff.decided"
	EventMeetingEnded              = "meeting.ended"
	EventKnowledgeCandidateCreated = "knowledge.candidate.created"
)

// Event 标准事件信封
type Event struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	TenantID   string `json:"tenant_id"`
	ActorType  string `json:"actor_type"`
	ActorID    string `json:"actor_id"`
	Payload    string `json:"payload"` // JSON string
	TraceID    string `json:"trace_id"`
	OccurredAt string `json:"occurred_at"`
}

// EventHandler 事件处理函数
type EventHandler func(ctx context.Context, event Event) error

// handlerEntry 处理器注册项
type handlerEntry struct {
	handler EventHandler
	name    string
}

// EventBus 事件总线（支持内存/Redis两种模式）
type EventBus struct {
	mu        sync.RWMutex
	handlers  map[string][]handlerEntry
	redisMode bool
}

var (
	eventBus     *EventBus
	eventBusOnce sync.Once
)

// GetEventBus 返回全局事件总线实例
func GetEventBus() *EventBus {
	eventBusOnce.Do(func() {
		eventBus = &EventBus{
			handlers: make(map[string][]handlerEntry),
		}
	})
	return eventBus
}

// Publish 发布事件到对应 channel
func (b *EventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	entries, ok := b.handlers[event.Type]
	b.mu.RUnlock()

	if !ok || len(entries) == 0 {
		slog.Debug("no handlers for event", "type", event.Type)
		return nil
	}

	for _, entry := range entries {
		func(h EventHandler, name string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event handler panic",
						"handler", name,
						"eventType", event.Type,
						"recover", r)
				}
			}()
			if err := h(ctx, event); err != nil {
				slog.Error("event handler failed",
					"handler", name,
					"eventType", event.Type,
					"error", err)
			}
		}(entry.handler, entry.name)
	}

	return nil
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	name := getHandlerName(handler)
	b.handlers[eventType] = append(b.handlers[eventType], handlerEntry{
		handler: handler,
		name:    name,
	})

	slog.Info("event subscriber registered", "eventType", eventType, "handler", name)
}

// NewEvent 创建标准事件
func (b *EventBus) NewEvent(eventType, tenantID, actorType, actorID string, payload any, traceID string) Event {
	var payloadStr string
	if payload != nil {
		b, err := json.Marshal(payload)
		if err == nil {
			payloadStr = string(b)
		}
	}

	return Event{
		ID:         utils.UUID(),
		Type:       eventType,
		TenantID:   tenantID,
		ActorType:  actorType,
		ActorID:    actorID,
		Payload:    payloadStr,
		TraceID:    traceID,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// getHandlerName 获取处理器名称（调试用）
func getHandlerName(h EventHandler) string {
	if h == nil {
		return "nil"
	}
	return fmt.Sprintf("%p", h)
}
