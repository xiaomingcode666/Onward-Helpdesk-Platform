package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const workflowEffectLeaseDuration = 2 * time.Minute

type workflowEffectStore struct{}

func (workflowEffectStore) Acquire(_ context.Context, req workflowexecutor.WorkflowEffectRequest) (workflowexecutor.WorkflowEffectLease, error) {
	now := time.Now()
	item := repositories.AIWorkflowEffectRepository.GetByIdempotencyKey(sqls.DB(), req.IdempotencyKey)
	if item == nil {
		lockedUntil := now.Add(workflowEffectLeaseDuration)
		item = &models.AIWorkflowEffect{
			TenantID:          req.TenantID,
			ProductID:         req.ProductID,
			WorkflowVersionID: req.WorkflowVersionID,
			AgentReleaseID:    req.AgentReleaseID,
			ConversationID:    req.ConversationID,
			MessageID:         req.MessageID,
			NodeID:            req.NodeID,
			EffectType:        req.EffectType,
			IdempotencyKey:    req.IdempotencyKey,
			Status:            models.AIWorkflowEffectStatusRunning,
			Attempt:           1,
			RequestData:       req.RequestData,
			LockedUntil:       &lockedUntil,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := repositories.AIWorkflowEffectRepository.Create(sqls.DB(), item); err == nil {
			return workflowExecutorEffectLease(item, true), nil
		}
		item = repositories.AIWorkflowEffectRepository.GetByIdempotencyKey(sqls.DB(), req.IdempotencyKey)
		if item == nil {
			return workflowexecutor.WorkflowEffectLease{}, fmt.Errorf("create workflow effect ledger failed")
		}
	}
	if item.Status == models.AIWorkflowEffectStatusSucceeded {
		return workflowExecutorEffectLease(item, false), nil
	}
	if item.LockedUntil != nil && item.LockedUntil.After(now) {
		return workflowExecutorEffectLease(item, false), nil
	}
	acquired, err := repositories.AIWorkflowEffectRepository.TryAcquire(sqls.DB(), item.ID, now, now.Add(workflowEffectLeaseDuration), req.RequestData)
	if err != nil {
		return workflowexecutor.WorkflowEffectLease{}, err
	}
	item = repositories.AIWorkflowEffectRepository.GetByIdempotencyKey(sqls.DB(), req.IdempotencyKey)
	return workflowExecutorEffectLease(item, acquired), nil
}

func (workflowEffectStore) Complete(_ context.Context, lease workflowexecutor.WorkflowEffectLease, req workflowexecutor.WorkflowEffectRequest, resultData string) error {
	now := time.Now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AIWorkflowEffectRepository.MarkSucceeded(ctx.Tx, lease.ID, lease.Attempt, resultData, now); err != nil {
			return err
		}
		eventKey := req.IdempotencyKey + ":succeeded"
		if repositories.DomainEventRepository.GetByIdempotencyKey(ctx.Tx, eventKey) != nil {
			return nil
		}
		payload, err := json.Marshal(map[string]any{
			"tenantId":          req.TenantID,
			"productId":         req.ProductID,
			"workflowVersionId": req.WorkflowVersionID,
			"agentReleaseId":    req.AgentReleaseID,
			"conversationId":    req.ConversationID,
			"messageId":         req.MessageID,
			"nodeId":            req.NodeID,
			"effectType":        req.EffectType,
			"result":            json.RawMessage(resultData),
		})
		if err != nil {
			return err
		}
		event := &models.DomainEvent{
			TenantID:       req.TenantID,
			TraceID:        req.IdempotencyKey,
			IdempotencyKey: eventKey,
			SchemaVersion:  1,
			EventType:      "ai.workflow.effect.succeeded",
			Payload:        string(payload),
			Source:         "ai_workflow",
			AggregateID:    fmt.Sprintf("%d", req.ConversationID),
			ActorType:      "ai_agent",
			CreatedAt:      now,
		}
		if err := repositories.DomainEventRepository.Create(ctx.Tx, event); err != nil {
			return err
		}
		return repositories.OutboxRecordRepository.Create(ctx.Tx, &models.OutboxRecord{
			EventID:    event.ID,
			EventType:  event.EventType,
			Status:     models.OutboxStatusPending,
			MaxRetries: 3,
			CreatedAt:  now,
		})
	})
}

func (workflowEffectStore) Fail(_ context.Context, lease workflowexecutor.WorkflowEffectLease, errorMessage string) error {
	if lease.ID <= 0 {
		return nil
	}
	return repositories.AIWorkflowEffectRepository.MarkFailed(sqls.DB(), lease.ID, lease.Attempt, errorMessage, time.Now())
}

func workflowExecutorEffectLease(item *models.AIWorkflowEffect, acquired bool) workflowexecutor.WorkflowEffectLease {
	if item == nil {
		return workflowexecutor.WorkflowEffectLease{}
	}
	return workflowexecutor.WorkflowEffectLease{
		ID:         item.ID,
		Attempt:    item.Attempt,
		Acquired:   acquired,
		Succeeded:  item.Status == models.AIWorkflowEffectStatusSucceeded,
		ResultData: item.ResultData,
	}
}
