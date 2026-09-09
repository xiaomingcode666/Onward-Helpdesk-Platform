package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
)

var AuditService = newAuditService()

func newAuditService() *auditService {
	return &auditService{}
}

type auditService struct{}

// RecordAuditInput 记录审计日志的输入参数
type RecordAuditInput struct {
	TenantID       int64
	ActorID        string
	ActorType      string // user / system / api_key
	Domain         string // ticket / meeting / diagnosis / knowledge / settings
	ResourceType   string
	ResourceID     string
	Action         string
	BeforeState    any
	AfterState     any
	IPAddress      string
	UserAgent      string
	RequestID      string
	SupportGrantID int64
	RiskLevel      string // low / medium / high / critical
}

// RecordAudit 记录审计日志
func (s *auditService) RecordAudit(ctx context.Context, in RecordAuditInput) error {
	err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		return s.RecordAuditTx(txCtx, in)
	})
	if err != nil {
		slog.Error("audit: failed to record audit log", "action", in.Action, "error", err)
		return err
	}

	return nil
}

// RecordAuditTx persists the audit log and its durable event in the caller's transaction.
func (s *auditService) RecordAuditTx(txCtx *sqls.TxContext, in RecordAuditInput) error {
	if txCtx == nil || txCtx.Tx == nil {
		return errors.New("audit transaction is required")
	}
	if in.RiskLevel == "" {
		in.RiskLevel = models.RiskLevelLow
	}

	beforeJSON := marshalState(in.BeforeState)
	afterJSON := marshalState(in.AfterState)

	log := &models.AuditLog{
		TenantID:       in.TenantID,
		ActorID:        in.ActorID,
		ActorType:      in.ActorType,
		Domain:         in.Domain,
		ResourceType:   in.ResourceType,
		ResourceID:     in.ResourceID,
		Action:         in.Action,
		BeforeState:    beforeJSON,
		AfterState:     afterJSON,
		IPAddress:      in.IPAddress,
		UserAgent:      in.UserAgent,
		RequestID:      in.RequestID,
		SupportGrantID: in.SupportGrantID,
		RiskLevel:      in.RiskLevel,
		Status:         models.AuditStatusSuccess,
		CreatedAt:      time.Now(),
	}

	if err := txCtx.Tx.Create(log).Error; err != nil {
		return err
	}
	eventID := "tenant:" + strconv.FormatInt(in.TenantID, 10) + ":audit.log.created:" + strconv.FormatInt(log.ID, 10)
	_, err := eventbus.EnqueueTx(txCtx.Tx, eventbus.DurableEvent{
		TenantID:       in.TenantID,
		TraceID:        in.RequestID,
		IdempotencyKey: eventID,
		EventType:      events.EventAuditLogCreated,
		Payload: events.AuditLogCreatedEvent{
			EventID:    eventID,
			AuditLogID: log.ID,
			TenantID:   in.TenantID,
			ActorID:    in.ActorID,
			Action:     in.Action,
			Domain:     in.Domain,
		},
		Source:      "audit_service",
		AggregateID: strconv.FormatInt(log.ID, 10),
		ActorID:     in.ActorID,
		ActorType:   in.ActorType,
		CreatedAt:   log.CreatedAt,
	})
	if err == nil && txCtx.RegisterCallback != nil {
		txCtx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
	}
	return err
}

// RecordAuditFailure 记录失败的审计日志（权限拦截等场景）
func (s *auditService) RecordAuditFailure(ctx context.Context, in RecordAuditInput, failReason string) error {
	if in.RiskLevel == "" {
		in.RiskLevel = models.RiskLevelMedium
	}

	beforeJSON := marshalState(in.BeforeState)

	log := &models.AuditLog{
		TenantID:       in.TenantID,
		ActorID:        in.ActorID,
		ActorType:      in.ActorType,
		Domain:         in.Domain,
		ResourceType:   in.ResourceType,
		ResourceID:     in.ResourceID,
		Action:         in.Action,
		BeforeState:    beforeJSON,
		AfterState:     failReason,
		IPAddress:      in.IPAddress,
		UserAgent:      in.UserAgent,
		RequestID:      in.RequestID,
		SupportGrantID: in.SupportGrantID,
		RiskLevel:      in.RiskLevel,
		Status:         models.AuditStatusFailure,
		CreatedAt:      time.Now(),
	}

	if err := sqls.DB().Create(log).Error; err != nil {
		slog.Error("audit: failed to record audit failure log", "action", in.Action, "error", err)
		return err
	}

	return nil
}

// QueryAuditLogs 查询审计日志（支持按租户、操作人、操作类型、时间范围过滤）
func (s *auditService) QueryAuditLogs(ctx context.Context, tenantID int64, filter map[string]any, page, pageSize int) ([]models.AuditLog, int64, error) {
	db := sqls.DB().Model(&models.AuditLog{})

	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	if action, ok := filter["action"]; ok {
		db = db.Where("action = ?", action)
	}
	if actorID, ok := filter["actor_id"]; ok {
		db = db.Where("actor_id = ?", actorID)
	}
	if domain, ok := filter["domain"]; ok {
		db = db.Where("domain = ?", domain)
	}
	if riskLevel, ok := filter["risk_level"]; ok {
		db = db.Where("risk_level = ?", riskLevel)
	}

	if startAt, ok := filter["start_at"]; ok {
		db = db.Where("created_at >= ?", startAt)
	}
	if endAt, ok := filter["end_at"]; ok {
		db = db.Where("created_at <= ?", endAt)
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	db.Count(&total)

	var logs []models.AuditLog
	db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	return logs, total, nil
}

// marshalState 将任意状态序列化为 JSON 字符串
func marshalState(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// 编译时检查
var _ = utils.UUID
