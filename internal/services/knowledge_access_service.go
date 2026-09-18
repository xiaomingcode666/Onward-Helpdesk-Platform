package services

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
)

const (
	KnowledgeAccessSubjectUser = "user"
	KnowledgeAccessSubjectTeam = "team"

	KnowledgeAccessLevelView    = "view"
	KnowledgeAccessLevelOperate = "operate"

	SupportStatusReady      = "ready"
	SupportStatusRestricted = "restricted_support"
	SupportStatusUnknown    = "unknown"

	SupportReasonMissingAccess       = "missing_access"
	SupportReasonKnowledgeNotMapped  = "knowledge_base_not_mapped"
	SupportReasonKnowledgeInactive   = "knowledge_base_inactive"
	SupportReasonAssigneeNotAssigned = "assignee_not_assigned"
)

func normalizeKnowledgeAccessSubjectType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case KnowledgeAccessSubjectUser:
		return KnowledgeAccessSubjectUser
	case KnowledgeAccessSubjectTeam:
		return KnowledgeAccessSubjectTeam
	default:
		return ""
	}
}

func normalizeKnowledgeAccessLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case KnowledgeAccessLevelView:
		return KnowledgeAccessLevelView
	default:
		return KnowledgeAccessLevelOperate
	}
}

func defaultTicketKnowledgeBaseDB(db *gorm.DB, tenantID int64) *models.KnowledgeBase {
	if db == nil || tenantID <= 0 || !db.Migrator().HasTable(&models.KnowledgeBase{}) {
		return nil
	}
	item := repositories.KnowledgeBaseRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("remark", tenantDefaultKnowledgeBaseRemark).
		Eq("status", enums.StatusOk).
		Asc("id"))
	if item != nil {
		return item
	}
	return repositories.KnowledgeBaseRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("name", "企业通用知识库").
		Eq("status", enums.StatusOk).
		Asc("id"))
}

func ListKnowledgeAccessGrants(tenantID, knowledgeBaseID int64) ([]dto.EnterpriseKnowledgeAccessGrantDTO, error) {
	knowledgeBase := KnowledgeBaseService.Get(knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("知识库不存在")
	}
	items, err := repositories.KnowledgeAccessGrantRepository.FindByKnowledgeBase(sqls.DB(), tenantID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.EnterpriseKnowledgeAccessGrantDTO, 0, len(items))
	for _, item := range items {
		subjectName := ""
		switch item.SubjectType {
		case KnowledgeAccessSubjectTeam:
			if team := AgentTeamService.GetForTenant(item.SubjectID, tenantID); team != nil {
				subjectName = team.Name
			}
		case KnowledgeAccessSubjectUser:
			if user := repositories.UserRepository.Get(sqls.DB(), item.SubjectID); user != nil {
				subjectName = firstNonBlank(user.Nickname, user.Username)
			}
		}
		result = append(result, dto.EnterpriseKnowledgeAccessGrantDTO{
			ID:          item.ID,
			SubjectType: item.SubjectType,
			SubjectID:   item.SubjectID,
			SubjectName: subjectName,
			AccessLevel: item.AccessLevel,
			Note:        item.Note,
		})
	}
	return result, nil
}

func ReplaceKnowledgeAccessGrants(
	tenantID, knowledgeBaseID int64,
	requests []dto.EnterpriseKnowledgeAccessGrantRequest,
	operator *dto.AuthPrincipal,
) ([]dto.EnterpriseKnowledgeAccessGrantDTO, error) {
	if operator == nil || operator.TenantID != tenantID {
		return nil, errorsx.Forbidden("无权维护知识库访问授权")
	}
	knowledgeBase := KnowledgeBaseService.Get(knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("知识库不存在")
	}
	if len(requests) > 100 {
		return nil, errorsx.InvalidParam("知识库访问授权不能超过 100 项")
	}
	seen := map[string]bool{}
	items := make([]models.KnowledgeAccessGrant, 0, len(requests))
	for _, request := range requests {
		subjectType := normalizeKnowledgeAccessSubjectType(request.SubjectType)
		if subjectType == "" || request.SubjectID <= 0 {
			return nil, errorsx.InvalidParam("访问授权主体不合法")
		}
		key := subjectType + ":" + formatID(request.SubjectID)
		if seen[key] {
			return nil, errorsx.InvalidParam("访问授权主体不能重复")
		}
		seen[key] = true
		switch subjectType {
		case KnowledgeAccessSubjectTeam:
			if team := AgentTeamService.GetForTenant(request.SubjectID, tenantID); team == nil {
				return nil, errorsx.InvalidParam("访问授权团队不存在或不属于当前公司")
			}
		case KnowledgeAccessSubjectUser:
			user := repositories.UserRepository.Get(sqls.DB(), request.SubjectID)
			if user == nil || user.Status != enums.StatusOk {
				return nil, errorsx.InvalidParam("访问授权工程师不存在或已停用")
			}
			if repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), tenantID, request.SubjectID) == nil {
				return nil, errorsx.InvalidParam("访问授权工程师不属于当前公司")
			}
		}
		items = append(items, models.KnowledgeAccessGrant{
			TenantID:        tenantID,
			KnowledgeBaseID: knowledgeBaseID,
			SubjectType:     subjectType,
			SubjectID:       request.SubjectID,
			AccessLevel:     normalizeKnowledgeAccessLevel(request.AccessLevel),
			Note:            strings.TrimSpace(request.Note),
			Status:          enums.StatusOk,
			AuditFields:     utils.BuildAuditFields(operator),
		})
	}
	if err := repositories.KnowledgeAccessGrantRepository.ReplaceForKnowledgeBase(
		sqls.DB(),
		tenantID,
		knowledgeBaseID,
		items,
	); err != nil {
		return nil, err
	}
	if err := revaluateKnowledgeBaseTicketsDB(sqls.DB(), tenantID, knowledgeBaseID); err != nil {
		return nil, err
	}
	return ListKnowledgeAccessGrants(tenantID, knowledgeBaseID)
}

func knowledgeBaseSupportStatusDB(db *gorm.DB, tenantID, knowledgeBaseID int64) (string, string) {
	if db == nil || tenantID <= 0 || knowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeAccessGrant{}) {
		return SupportStatusUnknown, ""
	}
	var count int64
	now := time.Now()
	err := db.Model(&models.KnowledgeAccessGrant{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND status = ?", tenantID, knowledgeBaseID, enums.StatusOk).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_until IS NULL OR effective_until >= ?)", now).
		Count(&count).Error
	if err != nil {
		return SupportStatusUnknown, ""
	}
	if count == 0 {
		return SupportStatusRestricted, "未配置知识库访问授权"
	}
	return SupportStatusReady, ""
}

func EvaluateTicketSupportReadinessDB(
	db *gorm.DB,
	ticket *models.Ticket,
	teamID, assigneeID, actorID int64,
	now time.Time,
) error {
	if db == nil || ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 ||
		ticket.KnowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeBase{}) {
		return nil
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, ticket.KnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != ticket.TenantID {
		return updateTicketSupportReadinessDB(db, ticket, SupportStatusUnknown, SupportReasonKnowledgeNotMapped,
			"未识别到有效的知识库", actorID, now)
	}
	if knowledgeBase.Status != enums.StatusOk {
		return updateTicketSupportReadinessDB(db, ticket, SupportStatusRestricted, SupportReasonKnowledgeInactive,
			"知识库已停用，当前不能提供正常支持", actorID, now)
	}
	if assigneeID <= 0 && teamID <= 0 {
		return updateTicketSupportReadinessDB(db, ticket, SupportStatusUnknown, SupportReasonAssigneeNotAssigned,
			"尚未分配支持团队或工程师，无法检查知识库访问权限", actorID, now)
	}
	hasAccess, err := repositories.KnowledgeAccessGrantRepository.HasActiveGrant(
		db, ticket.TenantID, knowledgeBase.ID, assigneeID, teamID, now,
	)
	if err != nil {
		return err
	}
	if hasAccess {
		return updateTicketSupportReadinessDB(db, ticket, SupportStatusReady, "", "", actorID, now)
	}
	subjectName := ""
	if assigneeID > 0 {
		if user := repositories.UserRepository.Get(db, assigneeID); user != nil {
			subjectName = firstNonBlank(user.Nickname, user.Username)
		}
	}
	if subjectName == "" && teamID > 0 {
		if team := AgentTeamService.GetForTenant(teamID, ticket.TenantID); team != nil {
			subjectName = team.Name
		}
	}
	reason := "当前支持团队或工程师缺少知识库“" + knowledgeBase.Name + "”的访问权限"
	if subjectName != "" {
		reason = subjectName + " 缺少知识库“" + knowledgeBase.Name + "”的访问权限"
	}
	return updateTicketSupportReadinessDB(
		db, ticket, SupportStatusRestricted, SupportReasonMissingAccess, reason, actorID, now,
	)
}

func revaluateKnowledgeBaseTicketsDB(db *gorm.DB, tenantID, knowledgeBaseID int64) error {
	if db == nil || tenantID <= 0 || knowledgeBaseID <= 0 {
		return nil
	}
	tickets := []models.Ticket{}
	if err := db.Where(
		"tenant_id = ? AND knowledge_base_id = ? AND current_assignee_id > 0",
		tenantID,
		knowledgeBaseID,
	).Where("status NOT IN ?", []enums.TicketStatus{
		enums.TicketStatusClosed,
		enums.TicketStatusDone,
		enums.TicketStatusCancelled,
	}).Find(&tickets).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range tickets {
		if err := EvaluateTicketSupportReadinessDB(
			db,
			&tickets[i],
			tickets[i].CurrentTeamID,
			tickets[i].CurrentAssigneeID,
			0,
			now,
		); err != nil {
			return err
		}
	}
	return nil
}

func updateTicketSupportReadinessDB(
	db *gorm.DB,
	ticket *models.Ticket,
	status, reasonCode, reason string,
	actorID int64,
	now time.Time,
) error {
	previousStatus := ticket.SupportStatus
	previousReasonCode := ticket.SupportReasonCode
	previousReason := ticket.SupportReason
	ticket.SupportStatus = status
	ticket.SupportReasonCode = reasonCode
	ticket.SupportReason = reason
	ticket.SupportCheckedAt = &now
	if err := db.Model(&models.Ticket{}).
		Where("id = ? AND tenant_id = ?", ticket.ID, ticket.TenantID).
		Updates(map[string]any{
			"support_status":      status,
			"support_reason_code": reasonCode,
			"support_reason":      reason,
			"support_checked_at":  now,
			"updated_at":          now,
		}).Error; err != nil {
		return err
	}
	if previousStatus == status && previousReasonCode == reasonCode && previousReason == reason {
		return nil
	}
	metadata, _ := json.Marshal(map[string]any{
		"status":              status,
		"reason_code":         reasonCode,
		"reason":              reason,
		"knowledge_base_id":   ticket.KnowledgeBaseID,
		"current_assignee_id": ticket.CurrentAssigneeID,
		"current_team_id":     ticket.CurrentTeamID,
		"checked_at":          now.Format(time.RFC3339),
	})
	content := "支持状态检查：" + supportStatusLabel(status)
	if reason != "" {
		content += "，" + reason
	}
	return repositories.TicketProgressRepository.Create(db, &models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventSupportReadiness,
		Content:      content,
		MetadataJSON: string(metadata),
		AuthorID:     actorID,
		CreatedAt:    now,
	})
}

func supportStatusLabel(status string) string {
	switch status {
	case SupportStatusReady:
		return "支持正常"
	case SupportStatusRestricted:
		return "支持受限"
	default:
		return "未评估"
	}
}
