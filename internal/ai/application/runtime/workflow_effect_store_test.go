package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestWorkflowEffectStoreCompletesWithOutboxAndReusesResult(t *testing.T) {
	db := setupWorkflowEffectStoreDB(t)
	store := workflowEffectStore{}
	req := workflowEffectTestRequest()

	lease, err := store.Acquire(context.Background(), req)
	if err != nil || !lease.Acquired || lease.Attempt != 1 {
		t.Fatalf("acquire effect lease: lease=%#v err=%v", lease, err)
	}
	resultData := `{"ticketId":88,"created":true}`
	if err := store.Complete(context.Background(), lease, req, resultData); err != nil {
		t.Fatalf("complete effect: %v", err)
	}

	reused, err := store.Acquire(context.Background(), req)
	if err != nil {
		t.Fatalf("reacquire completed effect: %v", err)
	}
	if reused.Acquired || !reused.Succeeded || reused.ResultData != resultData {
		t.Fatalf("expected completed effect reuse, got %#v", reused)
	}
	var eventCount int64
	var outboxCount int64
	if err := db.Model(&models.DomainEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count domain events: %v", err)
	}
	if err := db.Model(&models.OutboxRecord{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox records: %v", err)
	}
	if eventCount != 1 || outboxCount != 1 {
		t.Fatalf("expected one atomic event/outbox pair, event=%d outbox=%d", eventCount, outboxCount)
	}
}

func TestWorkflowEffectStoreFailedLeaseCanBeRetried(t *testing.T) {
	setupWorkflowEffectStoreDB(t)
	store := workflowEffectStore{}
	req := workflowEffectTestRequest()
	lease, err := store.Acquire(context.Background(), req)
	if err != nil {
		t.Fatalf("acquire effect: %v", err)
	}
	if err := store.Fail(context.Background(), lease, "temporary failure"); err != nil {
		t.Fatalf("fail effect: %v", err)
	}
	retry, err := store.Acquire(context.Background(), req)
	if err != nil {
		t.Fatalf("reacquire failed effect: %v", err)
	}
	if !retry.Acquired || retry.Attempt != 2 {
		t.Fatalf("expected second acquired attempt, got %#v", retry)
	}
}

func TestWorkflowEffectStoreRejectsStaleLeaseCompletion(t *testing.T) {
	db := setupWorkflowEffectStoreDB(t)
	store := workflowEffectStore{}
	req := workflowEffectTestRequest()
	stale, err := store.Acquire(context.Background(), req)
	if err != nil {
		t.Fatalf("acquire first effect lease: %v", err)
	}
	if err := db.Model(&models.AIWorkflowEffect{}).Where("id = ?", stale.ID).
		Update("locked_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire first effect lease: %v", err)
	}
	current, err := store.Acquire(context.Background(), req)
	if err != nil || !current.Acquired || current.Attempt != 2 {
		t.Fatalf("acquire replacement lease: lease=%#v err=%v", current, err)
	}
	if err := store.Complete(context.Background(), stale, req, `{"ticketId":1}`); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale lease completion error, got %v", err)
	}
	if err := store.Complete(context.Background(), current, req, `{"ticketId":2}`); err != nil {
		t.Fatalf("complete current lease: %v", err)
	}
}

func setupWorkflowEffectStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflowEffect{}, &models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatalf("migrate workflow effect tables: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func workflowEffectTestRequest() workflowexecutor.WorkflowEffectRequest {
	return workflowexecutor.WorkflowEffectRequest{
		TenantID:          1,
		ProductID:         2,
		WorkflowVersionID: 3,
		ConversationID:    4,
		MessageID:         5,
		NodeID:            "create_ticket_1",
		EffectType:        "create_ticket",
		IdempotencyKey:    "workflow-effect:1:4:5:3:create_ticket_1",
		RequestData:       `{"confirmed":true}`,
	}
}
