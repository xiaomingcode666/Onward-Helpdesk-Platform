package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

// ============================================================
// 升级数据模型
// ============================================================

// EscalationRule 升级规则
type EscalationRule struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID    string    `gorm:"type:varchar(36);index;not null"`
	Name        string    `gorm:"type:varchar(128);not null;default:''"`
	Priority    string    `gorm:"type:varchar(16);not null;default:'';index"` // 适用工单优先级
	HoursIdle   int       `gorm:"type:int;not null;default:0"`                // 工单闲置多少小时后升级
	TargetLevel int       `gorm:"type:int;not null;default:1"`                // 升级到哪个层级（1/2/3/4）
	NotifyRoles string    `gorm:"type:text;not null;default:'[]'"`            // JSON array of role IDs
	AutoAssign  bool      `gorm:"not null;default:false"`
	Status      string    `gorm:"type:varchar(20);not null;default:'active';index"`
	CreatedAt   time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt   time.Time `gorm:"type:timestamp;not null;index"`
}

func (EscalationRule) TableName() string {
	return "escalation_rules"
}

// TicketEscalation 工单升级记录
type TicketEscalation struct {
	ID               string     `gorm:"primaryKey;type:varchar(36)"`
	TicketID         string     `gorm:"type:varchar(36);index;not null"`
	TenantID         string     `gorm:"type:varchar(36);index;not null"`
	EscalationRuleID string     `gorm:"type:varchar(36);index;not null;default:''"`
	FromLevel        int        `gorm:"type:int;not null;default:0"`
	ToLevel          int        `gorm:"type:int;not null;default:0"`
	Reason           string     `gorm:"type:varchar(500);not null;default:''"`
	TriggerType      string     `gorm:"type:varchar(32);not null;default:'';index"`        // sla_violation / idle_timeout / manual / customer_request
	Status           string     `gorm:"type:varchar(20);not null;default:'pending';index"` // pending / accepted / completed / rejected
	EscalatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	ResolvedAt       *time.Time `gorm:"type:timestamp;index"`
	CreatedAt        time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt        time.Time  `gorm:"type:timestamp;not null;index"`
}

func (TicketEscalation) TableName() string {
	return "ticket_escalations"
}

// ============================================================
// Escalation Service
// ============================================================

var EscalationService = newEscalationService()

func newEscalationService() *escalationService {
	return &escalationService{}
}

type escalationService struct{}

// CreateEscalationRuleInput 创建升级规则的输入参数
type CreateEscalationRuleInput struct {
	TenantID    string
	Name        string
	Priority    string   // 适用工单优先级
	HoursIdle   int      // 工单闲置多少小时后升级
	TargetLevel int      // 升级到哪个层级（1/2/3/4）
	NotifyRoles []string // 通知角色 ID 列表
	AutoAssign  bool     // 是否自动派单
}

// CreateEscalationRule 创建升级规则
func (s *escalationService) CreateEscalationRule(in CreateEscalationRuleInput) (*EscalationRule, error) {
	if in.TenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}
	if in.Name == "" {
		return nil, errorsx.InvalidParam("escalation rule name is required")
	}
	if in.TargetLevel < 1 || in.TargetLevel > 4 {
		return nil, errorsx.InvalidParam("target_level must be 1-4")
	}
	if in.HoursIdle <= 0 {
		return nil, errorsx.InvalidParam("hours_idle must be > 0")
	}

	notifyRolesJSON := "[]"
	if len(in.NotifyRoles) > 0 {
		b, err := json.Marshal(in.NotifyRoles)
		if err != nil {
			return nil, errorsx.InvalidParam("invalid notify_roles")
		}
		notifyRolesJSON = string(b)
	}

	rule := &EscalationRule{
		ID:          uuid.NewString(),
		TenantID:    in.TenantID,
		Name:        in.Name,
		Priority:    in.Priority,
		HoursIdle:   in.HoursIdle,
		TargetLevel: in.TargetLevel,
		NotifyRoles: notifyRolesJSON,
		AutoAssign:  in.AutoAssign,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := sqls.DB().Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

// GetEscalationRule 根据 ID 获取升级规则
func (s *escalationService) GetEscalationRule(id string) *EscalationRule {
	var rule EscalationRule
	if err := sqls.DB().Where("id = ?", id).First(&rule).Error; err != nil {
		return nil
	}
	return &rule
}

// FindEscalationRulesByTenant 查询租户下的升级规则列表
func (s *escalationService) FindEscalationRulesByTenant(tenantID string) []EscalationRule {
	var rules []EscalationRule
	sqls.DB().Where("tenant_id = ?", tenantID).Order("created_at desc").Find(&rules)
	return rules
}

// FindActiveEscalationRules 查询所有生效中的升级规则
func (s *escalationService) FindActiveEscalationRules() []EscalationRule {
	var rules []EscalationRule
	sqls.DB().Where("status = 'active'").Order("updated_at DESC").Find(&rules)
	return rules
}

// CheckAndEscalate 检测所有超时工单并执行升级
// 由定时任务定期调用：扫描闲置超时的工单，执行自动升级。
func (s *escalationService) CheckAndEscalate() ([]TicketEscalation, error) {
	var escalations []TicketEscalation

	rules := s.FindActiveEscalationRules()
	if len(rules) == 0 {
		return escalations, nil
	}

	now := time.Now()
	processedTickets := make(map[int64]struct{})

	for _, rule := range rules {
		tenantID, err := strconv.ParseInt(rule.TenantID, 10, 64)
		if err != nil || tenantID <= 0 {
			continue
		}
		var tickets []models.Ticket
		if err := sqls.DB().
			Where("tenant_id = ?", tenantID).
			Where("status NOT IN ('closed', 'cancelled', 'done')").
			Where("updated_at < ?", now.Add(-time.Duration(rule.HoursIdle)*time.Hour)).
			Find(&tickets).Error; err != nil {
			return nil, err
		}

		for _, ticket := range tickets {
			if _, exists := processedTickets[ticket.ID]; exists || ticketSLAPriority(ticket) != rule.Priority {
				continue
			}
			processedTickets[ticket.ID] = struct{}{}
			var existingCount int64
			if err := sqls.DB().Model(&TicketEscalation{}).
				Where("tenant_id = ? AND ticket_id = ? AND to_level = ? AND status IN ('pending', 'accepted')",
					rule.TenantID, formatID(ticket.ID), rule.TargetLevel).
				Count(&existingCount).Error; err != nil {
				return nil, err
			}
			if existingCount > 0 {
				continue
			}

			escalation, err := s.EscalateTicket(
				formatID(ticket.ID),
				rule.TenantID,
				rule.ID,
				0,
				rule.TargetLevel,
				"idle_timeout",
				fmt.Sprintf("工单闲置超过%d小时，自动升级", rule.HoursIdle),
			)
			if err != nil {
				continue
			}
			escalations = append(escalations, *escalation)
		}
	}

	return escalations, nil
}

// EscalateTicket 升级单个工单
// 记录升级记录到 ticket_escalations 表。
func (s *escalationService) EscalateTicket(ticketID, tenantID, ruleID string, fromLevel, toLevel int, triggerType, reason string) (*TicketEscalation, error) {
	if ticketID == "" {
		return nil, errorsx.InvalidParam("ticket_id is required")
	}
	if toLevel < 1 || toLevel > 4 {
		return nil, errorsx.InvalidParam("to_level must be 1-4")
	}
	if !isValidTriggerType(triggerType) {
		return nil, errorsx.InvalidParam("invalid trigger_type: " + triggerType)
	}
	parsedTicketID := parseID(ticketID)
	parsedTenantID := parseID(tenantID)
	if parsedTicketID <= 0 || parsedTenantID <= 0 {
		return nil, errorsx.InvalidParam("valid ticket_id and tenant_id are required")
	}
	var ticketCount int64
	if err := sqls.DB().Model(&models.Ticket{}).
		Where("id = ? AND tenant_id = ?", parsedTicketID, parsedTenantID).
		Count(&ticketCount).Error; err != nil {
		return nil, err
	}
	if ticketCount != 1 {
		return nil, errorsx.InvalidParam("ticket does not belong to tenant")
	}

	escalation := &TicketEscalation{
		ID:               uuid.NewString(),
		TicketID:         ticketID,
		TenantID:         tenantID,
		EscalationRuleID: ruleID,
		FromLevel:        fromLevel,
		ToLevel:          toLevel,
		Reason:           reason,
		TriggerType:      triggerType,
		Status:           "pending",
		EscalatedAt:      time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := sqls.DB().Create(escalation).Error; err != nil {
		return nil, err
	}

	return escalation, nil
}

// GetTicketEscalations 获取工单的升级记录
func (s *escalationService) GetTicketEscalations(ticketID string) []TicketEscalation {
	var records []TicketEscalation
	sqls.DB().Where("ticket_id = ?", ticketID).Order("escalated_at desc").Find(&records)
	return records
}

// AcceptEscalation 接受升级（目标负责人确认接单）
func (s *escalationService) AcceptEscalation(escalationID string) error {
	now := time.Now()
	return sqls.DB().Model(&TicketEscalation{}).Where("id = ?", escalationID).Updates(map[string]any{
		"status":     "accepted",
		"updated_at": now,
	}).Error
}

// CompleteEscalation 完成升级
func (s *escalationService) CompleteEscalation(escalationID string) error {
	now := time.Now()
	return sqls.DB().Model(&TicketEscalation{}).Where("id = ?", escalationID).Updates(map[string]any{
		"status":      "completed",
		"resolved_at": now,
		"updated_at":  now,
	}).Error
}

// ============================================================
// 辅助函数
// ============================================================

func isValidTriggerType(t string) bool {
	switch t {
	case "sla_violation", "idle_timeout", "manual", "customer_request":
		return true
	default:
		return false
	}
}
