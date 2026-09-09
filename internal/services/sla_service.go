package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// SLA 数据模型
// ============================================================

// SLAPolicy SLA 策略定义
type SLAPolicy struct {
	ID                string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID          string    `gorm:"type:varchar(36);index;not null"`
	Name              string    `gorm:"type:varchar(128);not null;default:''"`
	Priority          string    `gorm:"type:varchar(16);not null;default:'';index"` // p0/p1/p2/p3/p4
	FRTMinutes        int       `gorm:"type:int;not null;default:0"`                // 首次响应时间目标（分钟）
	AssignmentMinutes int       `gorm:"type:int;not null;default:0"`                // 接单时间目标
	ResolutionMinutes int       `gorm:"type:int;not null;default:0"`                // 处理时间目标
	CalendarID        string    `gorm:"type:varchar(36);not null;default:'';index"` // 关联服务日历
	Status            string    `gorm:"type:varchar(20);not null;default:'active';index"`
	CreatedAt         time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt         time.Time `gorm:"type:timestamp;not null;index"`
}

func (SLAPolicy) TableName() string {
	return "sla_policies"
}

// SLAPauseRecord SLA 暂停记录
type SLAPauseRecord struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID  string     `gorm:"type:varchar(36);index;not null;default:''"`
	TicketID  string     `gorm:"type:varchar(36);index;not null"`
	Reason    string     `gorm:"type:varchar(32);not null;default:'';index"` // waiting_customer/waiting_parts/scheduled/on_hold
	PausedAt  time.Time  `gorm:"type:timestamp;not null;index"`
	ResumedAt *time.Time `gorm:"type:timestamp;index"`
	Duration  int64      `gorm:"type:bigint;not null;default:0"` // 暂停时长（秒）
	CreatedAt time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;index"`
}

func (SLAPauseRecord) TableName() string {
	return "sla_pause_records"
}

// SLAViolation SLA 违规记录
type SLAViolation struct {
	ID            string     `gorm:"primaryKey;type:varchar(36)"`
	TicketID      string     `gorm:"type:varchar(36);index;not null"`
	TenantID      string     `gorm:"type:varchar(36);index;not null"`
	ViolationType string     `gorm:"type:varchar(20);not null;default:'';index"` // frt/assignment/resolution
	TargetMinutes int        `gorm:"type:int;not null;default:0"`
	ActualMinutes int        `gorm:"type:int;not null;default:0"`
	Severity      string     `gorm:"type:varchar(20);not null;default:'warning';index"` // warning/breach/critical
	EscalatedAt   *time.Time `gorm:"type:timestamp;index"`
	ResolvedAt    *time.Time `gorm:"type:timestamp;index"`
	CreatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
}

func (SLAViolation) TableName() string {
	return "sla_violations"
}

// ServiceCalendar 服务日历
type ServiceCalendar struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID       string    `gorm:"type:varchar(36);index;not null"`
	Name           string    `gorm:"type:varchar(128);not null;default:''"`
	Timezone       string    `gorm:"type:varchar(64);not null;default:'UTC'"`
	WorkDays       string    `gorm:"column:work_days;type:text;not null;default:'[1,2,3,4,5]'"` // JSON array [1,2,3,4,5]
	WorkHoursStart string    `gorm:"type:varchar(5);not null;default:'09:00'"`                  // "09:00"
	WorkHoursEnd   string    `gorm:"type:varchar(5);not null;default:'18:00'"`                  // "18:00"
	Status         string    `gorm:"type:varchar(20);not null;default:'active';index"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

func (ServiceCalendar) TableName() string {
	return "service_calendars"
}

// ============================================================
// 时间线条目
// ============================================================

// SLATimelineEntry SLA 时间线上的一条记录
type SLATimelineEntry struct {
	Type      string    `json:"type"`      // frt_start / frt_complete / assignment_start / assignment_complete / resolution_start / resolution_complete / pause / resume / breach
	Label     string    `json:"label"`     // 中文描述
	TakenAt   time.Time `json:"takenAt"`   // 发生时间
	Remaining int       `json:"remaining"` // 剩余时间（分钟），负数表示已超时
	Details   string    `json:"details,omitempty"`
}

// SLATimeline 工单 SLA 时间线
type SLATimeline struct {
	TicketID     string             `json:"ticketId"`
	Paused       bool               `json:"paused"`
	Entries      []SLATimelineEntry `json:"entries"`
	PauseRecords []SLAPauseRecord   `json:"pauseRecords,omitempty"`
}

// ============================================================
// SLA Service
// ============================================================

var SLAService = newSLAService()

func newSLAService() *slaService {
	return &slaService{}
}

type slaService struct{}

// CreateSLAPolicyInput 创建 SLA 策略的输入参数
type CreateSLAPolicyInput struct {
	TenantID          string
	Name              string
	Priority          string // p0/p1/p2/p3/p4
	FRTMinutes        int
	AssignmentMinutes int
	ResolutionMinutes int
	CalendarID        string
}

// CreateSLAPolicy 创建 SLA 策略
func (s *slaService) CreateSLAPolicy(in CreateSLAPolicyInput) (*SLAPolicy, error) {
	if in.TenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}
	if in.Name == "" {
		return nil, errorsx.InvalidParam("sla policy name is required")
	}
	if !isValidPriority(in.Priority) {
		return nil, errorsx.InvalidParam("priority must be p0/p1/p2/p3/p4")
	}
	if in.FRTMinutes < 0 || in.AssignmentMinutes < 0 || in.ResolutionMinutes < 0 {
		return nil, errorsx.InvalidParam("sla targets must not be negative")
	}
	if in.FRTMinutes == 0 && in.AssignmentMinutes == 0 && in.ResolutionMinutes == 0 {
		return nil, errorsx.InvalidParam("at least one SLA target must be set")
	}
	if in.CalendarID != "" && s.GetServiceCalendarForTenant(in.TenantID, in.CalendarID) == nil {
		return nil, errorsx.InvalidParam("service calendar not found for this tenant")
	}
	if existing := s.FindActiveSLAPolicyByPriority(in.TenantID, in.Priority); existing != nil {
		return nil, errorsx.BusinessError(1, "an active SLA policy already exists for this priority")
	}

	policy := &SLAPolicy{
		ID:                uuid.NewString(),
		TenantID:          in.TenantID,
		Name:              in.Name,
		Priority:          in.Priority,
		FRTMinutes:        in.FRTMinutes,
		AssignmentMinutes: in.AssignmentMinutes,
		ResolutionMinutes: in.ResolutionMinutes,
		CalendarID:        in.CalendarID,
		Status:            "active",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := sqls.DB().Create(policy).Error; err != nil {
		return nil, err
	}
	return policy, nil
}

// GetSLAPolicy 根据 ID 获取 SLA 策略
func (s *slaService) GetSLAPolicy(id string) *SLAPolicy {
	var policy SLAPolicy
	if err := sqls.DB().Where("id = ?", id).First(&policy).Error; err != nil {
		return nil
	}
	return &policy
}

// GetSLAPolicyForTenant returns a policy only when it belongs to the tenant.
func (s *slaService) GetSLAPolicyForTenant(tenantID, id string) *SLAPolicy {
	if tenantID == "" || id == "" {
		return nil
	}
	var policy SLAPolicy
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, id).First(&policy).Error; err != nil {
		return nil
	}
	return &policy
}

// UpdateSLAPolicyInput 更新 SLA 策略的输入参数
type UpdateSLAPolicyInput struct {
	Name              *string `json:"name,omitempty"`
	Priority          *string `json:"priority,omitempty"`
	FRTMinutes        *int    `json:"frtMinutes,omitempty"`
	AssignmentMinutes *int    `json:"assignmentMinutes,omitempty"`
	ResolutionMinutes *int    `json:"resolutionMinutes,omitempty"`
	CalendarID        *string `json:"calendarId,omitempty"`
}

// UpdateSLAPolicy 更新 SLA 策略
func (s *slaService) UpdateSLAPolicy(id string, in UpdateSLAPolicyInput) (*SLAPolicy, error) {
	if id == "" {
		return nil, errorsx.InvalidParam("policy id is required")
	}
	policy := s.GetSLAPolicy(id)
	if policy == nil {
		return nil, errorsx.BusinessError(404, "sla policy not found")
	}
	return s.UpdateSLAPolicyForTenant(policy.TenantID, id, in)
}

// UpdateSLAPolicyForTenant updates a policy without allowing cross-tenant IDs.
func (s *slaService) UpdateSLAPolicyForTenant(tenantID, id string, in UpdateSLAPolicyInput) (*SLAPolicy, error) {
	if tenantID == "" || id == "" {
		return nil, errorsx.InvalidParam("tenant and policy id are required")
	}
	policy := s.GetSLAPolicyForTenant(tenantID, id)
	if policy == nil {
		return nil, errorsx.BusinessError(404, "sla policy not found")
	}

	updates := map[string]any{"updated_at": time.Now()}
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return nil, errorsx.InvalidParam("sla policy name is required")
		}
		updates["name"] = *in.Name
	}
	if in.Priority != nil {
		if !isValidPriority(*in.Priority) {
			return nil, errorsx.InvalidParam("priority must be p0/p1/p2/p3/p4")
		}
		updates["priority"] = *in.Priority
	}
	if in.FRTMinutes != nil {
		if *in.FRTMinutes < 0 {
			return nil, errorsx.InvalidParam("frt_minutes must not be negative")
		}
		updates["frt_minutes"] = *in.FRTMinutes
	}
	if in.AssignmentMinutes != nil {
		if *in.AssignmentMinutes < 0 {
			return nil, errorsx.InvalidParam("assignment_minutes must not be negative")
		}
		updates["assignment_minutes"] = *in.AssignmentMinutes
	}
	if in.ResolutionMinutes != nil {
		if *in.ResolutionMinutes < 0 {
			return nil, errorsx.InvalidParam("resolution_minutes must not be negative")
		}
		updates["resolution_minutes"] = *in.ResolutionMinutes
	}
	if in.CalendarID != nil {
		if *in.CalendarID != "" && s.GetServiceCalendarForTenant(tenantID, *in.CalendarID) == nil {
			return nil, errorsx.InvalidParam("service calendar not found for this tenant")
		}
		updates["calendar_id"] = *in.CalendarID
	}

	frtMinutes := policy.FRTMinutes
	assignmentMinutes := policy.AssignmentMinutes
	resolutionMinutes := policy.ResolutionMinutes
	if in.FRTMinutes != nil {
		frtMinutes = *in.FRTMinutes
	}
	if in.AssignmentMinutes != nil {
		assignmentMinutes = *in.AssignmentMinutes
	}
	if in.ResolutionMinutes != nil {
		resolutionMinutes = *in.ResolutionMinutes
	}
	if frtMinutes == 0 && assignmentMinutes == 0 && resolutionMinutes == 0 {
		return nil, errorsx.InvalidParam("at least one SLA target must be set")
	}
	priority := policy.Priority
	if in.Priority != nil {
		priority = *in.Priority
	}
	if policy.Status == "active" {
		var count int64
		sqls.DB().Model(&SLAPolicy{}).
			Where("tenant_id = ? AND priority = ? AND status = 'active' AND id <> ?", tenantID, priority, id).
			Count(&count)
		if count > 0 {
			return nil, errorsx.BusinessError(1, "an active SLA policy already exists for this priority")
		}
	}

	if err := sqls.DB().Model(&SLAPolicy{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetSLAPolicyForTenant(tenantID, id), nil
}

// ToggleSLAPolicy 切换 SLA 策略的启用/停用状态
func (s *slaService) ToggleSLAPolicy(id string) (*SLAPolicy, error) {
	if id == "" {
		return nil, errorsx.InvalidParam("policy id is required")
	}
	policy := s.GetSLAPolicy(id)
	if policy == nil {
		return nil, errorsx.BusinessError(404, "sla policy not found")
	}
	return s.ToggleSLAPolicyForTenant(policy.TenantID, id)
}

// ToggleSLAPolicyForTenant changes status only within the requested tenant.
func (s *slaService) ToggleSLAPolicyForTenant(tenantID, id string) (*SLAPolicy, error) {
	if tenantID == "" || id == "" {
		return nil, errorsx.InvalidParam("tenant and policy id are required")
	}
	policy := s.GetSLAPolicyForTenant(tenantID, id)
	if policy == nil {
		return nil, errorsx.BusinessError(404, "sla policy not found")
	}

	newStatus := "inactive"
	if policy.Status != "active" {
		newStatus = "active"
		if existing := s.FindActiveSLAPolicyByPriority(tenantID, policy.Priority); existing != nil && existing.ID != policy.ID {
			return nil, errorsx.BusinessError(1, "an active SLA policy already exists for this priority")
		}
	}

	now := time.Now()
	if err := sqls.DB().Model(&SLAPolicy{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(map[string]any{
		"status":     newStatus,
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	policy.Status = newStatus
	policy.UpdatedAt = now
	return policy, nil
}

// FindSLAPoliciesByTenant 查询租户下的 SLA 策略列表
func (s *slaService) FindSLAPoliciesByTenant(tenantID string) []SLAPolicy {
	var policies []SLAPolicy
	sqls.DB().Where("tenant_id = ?", tenantID).Order("created_at desc").Find(&policies)
	return policies
}

// FindActiveSLAPolicyByPriority 根据优先级查找生效中的 SLA 策略
func (s *slaService) FindActiveSLAPolicyByPriority(tenantID, priority string) *SLAPolicy {
	var policy SLAPolicy
	if err := sqls.DB().
		Where("tenant_id = ? AND priority = ? AND status = 'active'", tenantID, priority).
		First(&policy).Error; err != nil {
		return nil
	}
	return &policy
}

// CalculateSLADeadline 根据工单优先级 + SLA 策略 + 服务日历计算各阶段截止时间
//
// 返回一个 map，key 为阶段标识（frt/assignment/resolution），value 为截止时间。
// 如果未配置对应 SLA 策略则返回 nil 值。
func (s *slaService) CalculateSLADeadline(tenantID, priority string, startTime time.Time) map[string]time.Time {
	result := make(map[string]time.Time)

	policy := s.FindActiveSLAPolicyByPriority(tenantID, priority)
	if policy == nil {
		return result
	}

	var loc *time.Location
	var workDays []int
	var workStart, workEnd string
	if policy.CalendarID != "" {
		if cal := s.GetServiceCalendar(policy.CalendarID); cal != nil {
			loc, _ = time.LoadLocation(cal.Timezone)
			_ = json.Unmarshal([]byte(cal.WorkDays), &workDays)
			workStart = cal.WorkHoursStart
			workEnd = cal.WorkHoursEnd
		}
	}
	if loc == nil {
		loc = time.UTC
	}
	if len(workDays) == 0 {
		workDays = []int{1, 2, 3, 4, 5} // 默认周一到周五
	}
	if workStart == "" {
		workStart = "09:00"
	}
	if workEnd == "" {
		workEnd = "18:00"
	}

	if policy.FRTMinutes > 0 {
		result["frt"] = s.addWorkingMinutes(startTime, policy.FRTMinutes, workDays, workStart, workEnd, loc)
	}
	if policy.AssignmentMinutes > 0 {
		result["assignment"] = s.addWorkingMinutes(startTime, policy.AssignmentMinutes, workDays, workStart, workEnd, loc)
	}
	if policy.ResolutionMinutes > 0 {
		result["resolution"] = s.addWorkingMinutes(startTime, policy.ResolutionMinutes, workDays, workStart, workEnd, loc)
	}

	return result
}

// addWorkingMinutes 在工作日历的基础上增加 N 分钟（只计算工作时间）
// 此为简化实现，完整实现应考虑：节假日例外、跨周、跨月、暂停扣除
func (s *slaService) addWorkingMinutes(from time.Time, minutes int, workDays []int, workStart, workEnd string, loc *time.Location) time.Time {
	if minutes <= 0 {
		return from
	}

	if len(workDays) == 0 {
		return from.Add(time.Duration(minutes) * time.Minute)
	}

	t := from.In(loc)
	startH, startM := parseTime(workStart)
	endH, endM := parseTime(workEnd)
	workMinutesPerDay := (endH-startH)*60 + (endM - startM)
	if workMinutesPerDay <= 0 {
		workMinutesPerDay = 9 * 60 // 默认 9 小时
	}

	remaining := minutes
	for remaining > 0 {
		dayStart := time.Date(t.Year(), t.Month(), t.Day(), startH, startM, 0, 0, loc)
		dayEnd := time.Date(t.Year(), t.Month(), t.Day(), endH, endM, 0, 0, loc)

		isWorkDay := false
		for _, d := range workDays {
			if int(t.Weekday()) == d {
				isWorkDay = true
				break
			}
		}

		if !isWorkDay || t.Before(dayStart) {
			t = dayStart.AddDate(0, 0, 1)
			for {
				nextIsWorkDay := false
				for _, d := range workDays {
					if int(t.Weekday()) == d {
						nextIsWorkDay = true
						break
					}
				}
				if nextIsWorkDay {
					break
				}
				t = t.AddDate(0, 0, 1)
			}
			continue
		}

		if t.After(dayEnd) || !t.Before(dayEnd) {
			t = dayStart.AddDate(0, 0, 1)
			for {
				nextIsWorkDay := false
				for _, d := range workDays {
					if int(t.Weekday()) == d {
						nextIsWorkDay = true
						break
					}
				}
				if nextIsWorkDay {
					break
				}
				t = t.AddDate(0, 0, 1)
			}
			continue
		}

		available := int(dayEnd.Sub(t).Minutes())
		if available <= 0 {
			t = dayStart.AddDate(0, 0, 1)
			continue
		}
		if available >= remaining {
			t = t.Add(time.Duration(remaining) * time.Minute)
			remaining = 0
		} else {
			remaining -= available
			t = dayStart.AddDate(0, 0, 1)
		}
	}

	return t
}

// PauseSLA 暂停 SLA 计时
// 当工单处于等待外部因素状态（等待客户、等待备件等）时调用此方法。
func (s *slaService) PauseSLA(ticketID, reason string) (*SLAPauseRecord, error) {
	tenantID, err := s.resolveTicketTenant(ticketID)
	if err != nil {
		return nil, err
	}
	return s.PauseSLAForTenant(tenantID, ticketID, reason)
}

// PauseSLAForTenant pauses an owned ticket and treats duplicate requests as idempotent.
func (s *slaService) PauseSLAForTenant(tenantID, ticketID, reason string) (*SLAPauseRecord, error) {
	if tenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}
	if ticketID == "" {
		return nil, errorsx.InvalidParam("ticket_id is required")
	}
	if !isValidPauseReason(reason) {
		return nil, errorsx.InvalidParam("invalid pause reason: " + reason)
	}

	if _, err := s.getTicketForTenant(tenantID, ticketID); err != nil {
		return nil, err
	}
	if active := s.GetActivePauseForTenant(tenantID, ticketID); active != nil {
		return active, nil
	}

	record := &SLAPauseRecord{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		TicketID:  ticketID,
		Reason:    reason,
		PausedAt:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := sqls.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if active := s.GetActivePauseForTenant(tenantID, ticketID); active != nil {
			return active, nil
		}
		return nil, errorsx.BusinessError(1, "unable to pause SLA")
	}
	return record, nil
}

func (s *slaService) resolveTicketTenant(ticketID string) (string, error) {
	id, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil || id <= 0 {
		return "", errorsx.InvalidParam("invalid ticket_id")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("id = ?", id).First(&ticket).Error; err != nil {
		return "", errorsx.InvalidParam("ticket not found")
	}
	return strconv.FormatInt(ticket.TenantID, 10), nil
}

func (s *slaService) getTicketForTenant(tenantID, ticketID string) (*models.Ticket, error) {
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return nil, errorsx.InvalidParam("invalid tenant_id")
	}
	ticketIDInt, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil || ticketIDInt <= 0 {
		return nil, errorsx.InvalidParam("invalid ticket_id")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantIDInt, ticketIDInt).First(&ticket).Error; err != nil {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	return &ticket, nil
}

// ResumeSLA 恢复 SLA 计时
func (s *slaService) ResumeSLA(ticketID string) (*SLAPauseRecord, error) {
	tenantID, err := s.resolveTicketTenant(ticketID)
	if err != nil {
		return nil, err
	}
	return s.ResumeSLAForTenant(tenantID, ticketID)
}

// ResumeSLAForTenant resumes an owned ticket and is idempotent after success.
func (s *slaService) ResumeSLAForTenant(tenantID, ticketID string) (*SLAPauseRecord, error) {
	if tenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}
	if ticketID == "" {
		return nil, errorsx.InvalidParam("ticket_id is required")
	}
	if _, err := s.getTicketForTenant(tenantID, ticketID); err != nil {
		return nil, err
	}

	var record SLAPauseRecord
	if err := sqls.DB().
		Where("tenant_id = ? AND ticket_id = ? AND resumed_at IS NULL", tenantID, ticketID).
		Order("paused_at desc").
		First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if latest := s.getLatestPauseForTenant(tenantID, ticketID); latest != nil && latest.ResumedAt != nil {
				return latest, nil
			}
			return nil, errorsx.BusinessError(2, "no pause record found")
		}
		return nil, err
	}

	now := time.Now()
	duration := int64(now.Sub(record.PausedAt).Seconds())
	if duration < 0 {
		duration = 0
	}

	updates := map[string]any{
		"resumed_at": now,
		"duration":   duration,
		"updated_at": now,
	}

	result := sqls.DB().Model(&SLAPauseRecord{}).
		Where("tenant_id = ? AND id = ? AND resumed_at IS NULL", tenantID, record.ID).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if latest := s.getLatestPauseForTenant(tenantID, ticketID); latest != nil && latest.ResumedAt != nil {
			return latest, nil
		}
		return nil, errorsx.BusinessError(2, "no active pause record found")
	}

	record.ResumedAt = &now
	record.Duration = duration
	return &record, nil
}

func (s *slaService) getLatestPauseForTenant(tenantID, ticketID string) *SLAPauseRecord {
	var record SLAPauseRecord
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).Order("paused_at desc").First(&record).Error; err != nil {
		return nil
	}
	return &record
}

// GetActivePauseForTenant gets the active pause owned by a tenant.
func (s *slaService) GetActivePauseForTenant(tenantID, ticketID string) *SLAPauseRecord {
	var record SLAPauseRecord
	if err := sqls.DB().
		Where("tenant_id = ? AND ticket_id = ? AND resumed_at IS NULL", tenantID, ticketID).
		First(&record).Error; err != nil {
		return nil
	}
	return &record
}

// GetPauseRecordsForTenant returns pause history owned by a tenant.
func (s *slaService) GetPauseRecordsForTenant(tenantID, ticketID string) []SLAPauseRecord {
	var records []SLAPauseRecord
	sqls.DB().Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).Order("paused_at asc").Find(&records)
	return records
}

// GetSLATimelineForTenant returns a timeline only for an owned ticket.
func (s *slaService) GetSLATimelineForTenant(tenantID, ticketID string) (*SLATimeline, error) {
	ticket, err := s.getTicketForTenant(tenantID, ticketID)
	if err != nil {
		return nil, err
	}
	return s.buildSLATimeline(tenantID, ticketID, ticket)
}

// GetActivePause 获取工单当前活动的暂停记录
func (s *slaService) GetActivePause(ticketID string) *SLAPauseRecord {
	var record SLAPauseRecord
	if err := sqls.DB().
		Where("ticket_id = ? AND resumed_at IS NULL", ticketID).
		First(&record).Error; err != nil {
		return nil
	}
	return &record
}

// GetPauseRecords 获取工单的所有暂停记录
func (s *slaService) GetPauseRecords(ticketID string) []SLAPauseRecord {
	var records []SLAPauseRecord
	sqls.DB().Where("ticket_id = ?", ticketID).Order("paused_at asc").Find(&records)
	return records
}

// GetTotalPausedDuration 获取工单累计暂停时长（秒）
func (s *slaService) GetTotalPausedDuration(ticketID string) int64 {
	var total int64
	sqls.DB().Model(&SLAPauseRecord{}).
		Where("ticket_id = ?", ticketID).
		Select("COALESCE(SUM(duration), 0)").
		Scan(&total)

	active := s.GetActivePause(ticketID)
	if active != nil {
		total += int64(time.Since(active.PausedAt).Seconds())
	}

	return total
}

// GetTotalPausedDurationForTenant returns only pause time owned by the tenant.
func (s *slaService) GetTotalPausedDurationForTenant(tenantID, ticketID string) int64 {
	var total int64
	sqls.DB().Model(&SLAPauseRecord{}).
		Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Select("COALESCE(SUM(duration), 0)").
		Scan(&total)
	if active := s.GetActivePauseForTenant(tenantID, ticketID); active != nil {
		total += int64(time.Since(active.PausedAt).Seconds())
	}
	return total
}

// CheckSLAViolations 检测 SLA 违规
// 通常由定时任务调用，扫描所有未关闭的工单并检查是否超过 SLA 截止时间。
// 返回新检测到的违规记录列表。
func (s *slaService) CheckSLAViolations() ([]SLAViolation, error) {
	var violations []SLAViolation

	var policies []SLAPolicy
	if err := sqls.DB().Where("status = 'active'").Order("updated_at DESC").Find(&policies).Error; err != nil {
		return nil, err
	}

	if len(policies) == 0 {
		return violations, nil
	}

	now := time.Now()
	processedTickets := make(map[int64]struct{})

	for _, policy := range policies {
		tenantID, err := strconv.ParseInt(policy.TenantID, 10, 64)
		if err != nil || tenantID <= 0 {
			continue
		}
		var tickets []models.Ticket
		if err := sqls.DB().
			Where("tenant_id = ?", tenantID).
			Where("status NOT IN ('closed', 'cancelled', 'done')").
			Find(&tickets).Error; err != nil {
			return nil, err
		}

		for _, ticket := range tickets {
			if _, exists := processedTickets[ticket.ID]; exists || ticketSLAPriority(ticket) != policy.Priority {
				continue
			}
			processedTickets[ticket.ID] = struct{}{}
			items, err := s.checkSingleViolations(ticket, policy, now)
			if err != nil {
				return nil, err
			}
			violations = append(violations, items...)
		}
	}

	return violations, nil
}

type slaWarningTarget struct {
	violationType string
	targetMinutes int
	deadline      time.Time
}

// CheckSLAWarnings 在 SLA 到期前持续发布预警事件。事件按配置间隔分桶，
// 同一工单、同一 SLA 类型在一个提醒周期内只会生成一次通知。
func (s *slaService) CheckSLAWarnings() (int, error) {
	return s.checkSLAWarningsAt(time.Now())
}

func (s *slaService) checkSLAWarningsAt(now time.Time) (int, error) {
	var policies []SLAPolicy
	if err := sqls.DB().Where("status = 'active'").Order("updated_at DESC").Find(&policies).Error; err != nil {
		return 0, err
	}
	if len(policies) == 0 {
		return 0, nil
	}
	processedTickets := make(map[int64]struct{})
	warnings := 0
	for _, policy := range policies {
		tenantID, err := strconv.ParseInt(policy.TenantID, 10, 64)
		if err != nil || tenantID <= 0 {
			continue
		}
		var tickets []models.Ticket
		if err := sqls.DB().
			Where("tenant_id = ?", tenantID).
			Where("status NOT IN ('closed', 'cancelled', 'done')").
			Find(&tickets).Error; err != nil {
			return warnings, err
		}
		for _, ticket := range tickets {
			if _, exists := processedTickets[ticket.ID]; exists || ticketSLAPriority(ticket) != policy.Priority {
				continue
			}
			processedTickets[ticket.ID] = struct{}{}
			created, err := s.enqueueUpcomingWarnings(ticket, policy, now)
			if err != nil {
				return warnings, err
			}
			warnings += created
		}
	}
	return warnings, nil
}

func (s *slaService) enqueueUpcomingWarnings(ticket models.Ticket, policy SLAPolicy, now time.Time) (int, error) {
	ticketID := formatID(ticket.ID)
	tenantID := formatID(ticket.TenantID)
	if s.GetActivePauseForTenant(tenantID, ticketID) != nil {
		return 0, nil
	}
	pausedDuration := time.Duration(s.GetTotalPausedDurationForTenant(tenantID, ticketID)) * time.Second
	targets := make([]slaWarningTarget, 0, 3)
	if policy.FRTMinutes > 0 {
		firstResponseAt, err := s.firstAgentResponseAt(ticket)
		if err != nil {
			return 0, err
		}
		if firstResponseAt == nil {
			targets = append(targets, slaWarningTarget{
				violationType: "frt", targetMinutes: policy.FRTMinutes,
				deadline: ticket.CreatedAt.Add(time.Duration(policy.FRTMinutes) * time.Minute).Add(pausedDuration),
			})
		}
	}
	if policy.AssignmentMinutes > 0 && ticket.AcceptedAt == nil {
		targets = append(targets, slaWarningTarget{
			violationType: "assignment", targetMinutes: policy.AssignmentMinutes,
			deadline: ticket.CreatedAt.Add(time.Duration(policy.AssignmentMinutes) * time.Minute).Add(pausedDuration),
		})
	}
	if policy.ResolutionMinutes > 0 && ticket.ResolvedAt == nil {
		targetMinutes := policy.ResolutionMinutes
		deadline := ticket.CreatedAt.Add(time.Duration(targetMinutes) * time.Minute).Add(pausedDuration)
		if ticket.SLADueAt != nil {
			deadline = ticket.SLADueAt.Add(pausedDuration)
			targetMinutes = max(1, int(ticket.SLADueAt.Sub(ticket.CreatedAt).Minutes()))
		}
		targets = append(targets, slaWarningTarget{violationType: "resolution", targetMinutes: targetMinutes, deadline: deadline})
	}

	cfg := config.CurrentOrDefault().TicketDispatch.Normalized()
	lead := time.Duration(cfg.SLAWarningLeadMinutes) * time.Minute
	repeat := time.Duration(cfg.SLAWarningRepeatMinutes) * time.Minute
	bucket := now.Unix() / max(int64(1), int64(repeat/time.Second))
	createdCount := 0
	for _, target := range targets {
		remaining := target.deadline.Sub(now)
		if remaining <= 0 || remaining > lead {
			continue
		}
		actualMinutes := effectiveSLAMinutes(ticket.CreatedAt, now, pausedDuration)
		eventID := "tenant:" + tenantID + ":sla.risk:" + ticketID + ":" + target.violationType + ":" + strconv.FormatInt(bucket, 10)
		created := false
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			var err error
			created, err = eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
				TenantID:       ticket.TenantID,
				IdempotencyKey: eventID,
				EventType:      events.EventSLAWarning,
				Payload: events.SLAWarningEvent{
					EventID: eventID, TicketID: ticketID, TenantID: tenantID,
					ViolationType: target.violationType, Severity: "warning",
					TargetMinutes: target.targetMinutes, ActualMinutes: actualMinutes,
				},
				Source: "sla_service", AggregateID: ticketID, CreatedAt: now,
			})
			if err == nil && created && ctx.RegisterCallback != nil {
				ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
			}
			return err
		}); err != nil {
			return createdCount, err
		}
		if created {
			createdCount++
		}
		if target.violationType == "assignment" {
			s.triggerAssignmentRiskDispatchAction(ticket, "sla_assignment_risk", now)
		}
	}
	return createdCount, nil
}

// checkSingleViolations checks FRT, assignment and resolution independently so
// one breach never hides another breach on the same ticket.
func (s *slaService) checkSingleViolations(ticket models.Ticket, policy SLAPolicy, now time.Time) ([]SLAViolation, error) {
	existing := s.FindViolationsByTicket(formatID(ticket.ID))
	existingTypes := make(map[string]struct{}, len(existing))
	for _, violation := range existing {
		existingTypes[violation.ViolationType] = struct{}{}
	}

	ticketID := formatID(ticket.ID)
	tenantID := formatID(ticket.TenantID)
	pausedSeconds := s.GetTotalPausedDurationForTenant(tenantID, ticketID)
	pausedDuration := time.Duration(pausedSeconds) * time.Second
	violations := make([]SLAViolation, 0, 3)

	firstResponseAt, err := s.firstAgentResponseAt(ticket)
	if err != nil {
		return nil, err
	}
	frtEnd := now
	if firstResponseAt != nil {
		frtEnd = *firstResponseAt
	}
	if _, exists := existingTypes["frt"]; !exists && policy.FRTMinutes > 0 && frtEnd.After(ticket.CreatedAt.Add(time.Duration(policy.FRTMinutes)*time.Minute).Add(pausedDuration)) {
		violation, created, createErr := s.createViolation(ticketID, tenantID, "frt", policy.FRTMinutes, effectiveSLAMinutes(ticket.CreatedAt, frtEnd, pausedDuration), now)
		if createErr != nil {
			return nil, createErr
		}
		if created {
			violations = append(violations, *violation)
		}
	}

	// 接单(assignment) SLA 以工程师明确确认时间(accepted_at)为准；未接单时使用当前时间。
	// AssignedAt 只说明系统写入了负责人，不能证明负责人已经响应。
	assignmentEnd := now
	if ticket.AcceptedAt != nil {
		assignmentEnd = *ticket.AcceptedAt
	}
	if _, exists := existingTypes["assignment"]; !exists && policy.AssignmentMinutes > 0 && assignmentEnd.After(ticket.CreatedAt.Add(time.Duration(policy.AssignmentMinutes)*time.Minute).Add(pausedDuration)) {
		violation, created, createErr := s.createViolation(ticketID, tenantID, "assignment", policy.AssignmentMinutes, effectiveSLAMinutes(ticket.CreatedAt, assignmentEnd, pausedDuration), now)
		if createErr != nil {
			return nil, createErr
		}
		if created {
			violations = append(violations, *violation)
			s.triggerAssignmentDispatchAction(ticket, "sla_assignment_breached", now)
		}
	}

	resolutionEnd := now
	if ticket.ResolvedAt != nil {
		resolutionEnd = *ticket.ResolvedAt
	}
	resolutionTarget := policy.ResolutionMinutes
	resolutionDeadline := ticket.CreatedAt.Add(time.Duration(resolutionTarget) * time.Minute).Add(pausedDuration)
	if ticket.SLADueAt != nil {
		resolutionDeadline = ticket.SLADueAt.Add(pausedDuration)
		resolutionTarget = max(1, int(ticket.SLADueAt.Sub(ticket.CreatedAt).Minutes()))
	}
	if _, exists := existingTypes["resolution"]; !exists && resolutionTarget > 0 && resolutionEnd.After(resolutionDeadline) {
		violation, created, createErr := s.createViolation(ticketID, tenantID, "resolution", resolutionTarget, effectiveSLAMinutes(ticket.CreatedAt, resolutionEnd, pausedDuration), now)
		if createErr != nil {
			return nil, createErr
		}
		if created {
			violations = append(violations, *violation)
		}
	}

	return violations, nil
}

func (s *slaService) triggerAssignmentDispatchAction(ticket models.Ticket, reason string, now time.Time) {
	if ticket.ID <= 0 || ticket.AcceptedAt != nil {
		return
	}
	handled, err := TicketDispatchService.RecoverTicketAssignmentSLA(ticket.ID, now)
	if err != nil {
		slog.Warn("sla assignment dispatch action failed",
			"ticket_id", ticket.ID,
			"tenant_id", ticket.TenantID,
			"reason", reason,
			"error", err,
		)
		return
	}
	if handled {
		slog.Info("sla assignment dispatch action handled",
			"ticket_id", ticket.ID,
			"tenant_id", ticket.TenantID,
			"reason", reason,
		)
	}
}

func (s *slaService) triggerAssignmentRiskDispatchAction(ticket models.Ticket, reason string, now time.Time) {
	if ticket.CurrentAssigneeID > 0 {
		return
	}
	if ticket.DispatchDeferredUntil != nil && ticket.DispatchDeferredUntil.After(now) {
		return
	}
	s.triggerAssignmentDispatchAction(ticket, reason, now)
}

func (s *slaService) firstAgentResponseAt(ticket models.Ticket) (*time.Time, error) {
	if ticket.ConversationID <= 0 {
		return nil, nil
	}
	var message models.Message
	err := sqls.DB().
		Where("conversation_id = ? AND sender_type = ?", ticket.ConversationID, enums.IMSenderTypeAgent).
		Where("sent_at IS NOT NULL AND sent_at >= ?", ticket.CreatedAt).
		Order("sent_at ASC").
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return message.SentAt, nil
}

func (s *slaService) createViolation(ticketID, tenantID, violationType string, targetMinutes, actualMinutes int, now time.Time) (*SLAViolation, bool, error) {
	violation := &SLAViolation{
		ID:            uuid.NewString(),
		TicketID:      ticketID,
		TenantID:      tenantID,
		ViolationType: violationType,
		TargetMinutes: targetMinutes,
		ActualMinutes: actualMinutes,
		Severity:      classifySeverity(actualMinutes, targetMinutes),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	created := false
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		result := ctx.Tx.Clauses(clause.OnConflict{DoNothing: true}).Create(violation)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		created = true
		tenantIDValue, _ := strconv.ParseInt(tenantID, 10, 64)
		if tenantIDValue <= 0 {
			return nil
		}
		// 发布 SLA 告警事件：durable 事件经 outbox 触发站内/邮件通知（handler 已注册）。
		// EventID 绑定新建的 violation，重复扫描由上面的唯一约束吞掉。
		eventID := "tenant:" + tenantID + ":sla.warning:" + violation.ID
		createdEvent, err := eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
			TenantID:       tenantIDValue,
			IdempotencyKey: eventID,
			EventType:      events.EventSLAWarning,
			Payload: events.SLAWarningEvent{
				EventID:       eventID,
				TicketID:      ticketID,
				TenantID:      tenantID,
				ViolationType: violationType,
				Severity:      violation.Severity,
				TargetMinutes: targetMinutes,
				ActualMinutes: actualMinutes,
			},
			Source:      "sla_service",
			AggregateID: ticketID,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		if createdEvent && ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	if !created {
		return nil, false, nil
	}
	return violation, true, nil
}

func effectiveSLAMinutes(start, end time.Time, paused time.Duration) int {
	duration := end.Sub(start) - paused
	if duration < 0 {
		return 0
	}
	return int(duration.Minutes())
}

func ticketSLAPriority(ticket models.Ticket) string {
	priority := strings.ToLower(strings.TrimSpace(ticket.PriorityCode))
	if !isValidPriority(priority) {
		return "p2"
	}
	return priority
}

// GetSLATimeline 获取工单 SLA 时间线
func (s *slaService) GetSLATimeline(ticketID string) (*SLATimeline, error) {
	tenantID, err := s.resolveTicketTenant(ticketID)
	if err != nil {
		return nil, err
	}
	return s.GetSLATimelineForTenant(tenantID, ticketID)
}

func (s *slaService) buildSLATimeline(tenantID, ticketID string, ticket *models.Ticket) (*SLATimeline, error) {
	timeline := &SLATimeline{
		TicketID:     ticketID,
		Entries:      make([]SLATimelineEntry, 0),
		PauseRecords: s.GetPauseRecordsForTenant(tenantID, ticketID),
	}

	activePause := s.GetActivePauseForTenant(tenantID, ticketID)
	timeline.Paused = activePause != nil

	now := time.Now()

	timeline.Entries = append(timeline.Entries, SLATimelineEntry{
		Type:    "frt_start",
		Label:   "工单创建 - FRT 计时开始",
		TakenAt: ticket.CreatedAt,
	})

	if ticket.SLADueAt != nil {
		remaining := int(ticket.SLADueAt.Sub(now).Minutes())
		timeline.Entries = append(timeline.Entries, SLATimelineEntry{
			Type:      "resolution_deadline",
			Label:     "SLA 处理截止时间",
			TakenAt:   *ticket.SLADueAt,
			Remaining: remaining,
		})
	}

	for _, pr := range timeline.PauseRecords {
		entry := SLATimelineEntry{
			Type:  "pause",
			Label: "SLA 暂停 - " + pauseReasonLabel(pr.Reason),
		}
		if pr.ResumedAt != nil {
			entry.TakenAt = *pr.ResumedAt
		} else {
			entry.TakenAt = pr.PausedAt
		}
		timeline.Entries = append(timeline.Entries, entry)
	}

	violations := s.FindViolationsByTicketForTenant(tenantID, ticketID)
	for _, v := range violations {
		entry := SLATimelineEntry{
			Type:    "breach",
			Label:   "SLA 违规 - " + violationTypeLabel(v.ViolationType),
			TakenAt: v.CreatedAt,
			Details: v.Severity,
		}
		timeline.Entries = append(timeline.Entries, entry)
	}

	return timeline, nil
}

// FindViolationsByTicket 查询工单的所有 SLA 违规记录
func (s *slaService) FindViolationsByTicket(ticketID string) []SLAViolation {
	var violations []SLAViolation
	sqls.DB().Where("ticket_id = ?", ticketID).Order("created_at desc").Find(&violations)
	return violations
}

// FindViolationsByTicketForTenant returns violations only for an owned ticket.
func (s *slaService) FindViolationsByTicketForTenant(tenantID, ticketID string) []SLAViolation {
	var violations []SLAViolation
	sqls.DB().Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).Order("created_at desc").Find(&violations)
	return violations
}

// FindViolationsByTenant 查询租户下的所有 SLA 违规记录
func (s *slaService) FindViolationsByTenant(tenantID string) []SLAViolation {
	var violations []SLAViolation
	sqls.DB().Where("tenant_id = ?", tenantID).Order("created_at desc").Find(&violations)
	return violations
}

// ResolveViolation 标记违规记录为已解决
func (s *slaService) ResolveViolation(violationID string) error {
	now := time.Now()
	return sqls.DB().Model(&SLAViolation{}).Where("id = ?", violationID).Updates(map[string]any{
		"resolved_at": now,
		"updated_at":  now,
	}).Error
}

// --- 服务日历 ---

// CreateServiceCalendar 创建服务日历
func (s *slaService) CreateServiceCalendar(cal *ServiceCalendar) error {
	if cal.ID == "" {
		cal.ID = uuid.NewString()
	}
	if cal.Status == "" {
		cal.Status = "active"
	}
	cal.CreatedAt = time.Now()
	cal.UpdatedAt = time.Now()
	return sqls.DB().Create(cal).Error
}

// GetServiceCalendar 获取服务日历
func (s *slaService) GetServiceCalendar(id string) *ServiceCalendar {
	var cal ServiceCalendar
	if err := sqls.DB().Where("id = ?", id).First(&cal).Error; err != nil {
		return nil
	}
	return &cal
}

// GetServiceCalendarForTenant prevents cross-tenant calendar bindings.
func (s *slaService) GetServiceCalendarForTenant(tenantID, id string) *ServiceCalendar {
	if tenantID == "" || id == "" {
		return nil
	}
	var cal ServiceCalendar
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, id).First(&cal).Error; err != nil {
		return nil
	}
	return &cal
}

// FindServiceCalendarsByTenant 查询租户下的服务日历列表
func (s *slaService) FindServiceCalendarsByTenant(tenantID string) []ServiceCalendar {
	var cals []ServiceCalendar
	sqls.DB().Where("tenant_id = ?", tenantID).Order("created_at desc").Find(&cals)
	return cals
}

// ============================================================
// 辅助函数
// ============================================================

func isValidPriority(p string) bool {
	switch p {
	case "p0", "p1", "p2", "p3", "p4":
		return true
	default:
		return false
	}
}

func isValidPauseReason(reason string) bool {
	switch reason {
	case "waiting_customer", "waiting_parts", "scheduled", "on_hold":
		return true
	default:
		return false
	}
}

func classifySeverity(actualMinutes, targetMinutes int) string {
	if targetMinutes <= 0 {
		return "warning"
	}
	ratio := float64(actualMinutes) / float64(targetMinutes)
	switch {
	case ratio >= 1.5:
		return "critical"
	case ratio >= 1.0:
		return "breach"
	default:
		return "warning"
	}
}

func pauseReasonLabel(reason string) string {
	switch reason {
	case "waiting_customer":
		return "等待客户"
	case "waiting_parts":
		return "等待备件"
	case "scheduled":
		return "已排期"
	case "on_hold":
		return "暂挂"
	default:
		return reason
	}
}

func violationTypeLabel(vType string) string {
	switch vType {
	case "frt":
		return "首次响应"
	case "assignment":
		return "接单"
	case "resolution":
		return "处理"
	default:
		return vType
	}
}

// formatID 将 int64 ID 转换为 string（兼容 Ticket 模型的 int64 主键）
func formatID(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}

// parseID 将 string ID 解析为 int64（兼容 Ticket 模型的 int64 主键查询）
func parseID(id string) int64 {
	var n int64
	fmt.Sscanf(id, "%d", &n)
	return n
}

// parseTime 解析 "HH:MM" 格式的时间字符串
func parseTime(s string) (hour, min int) {
	fmt.Sscanf(s, "%d:%d", &hour, &min)
	return
}
