package services

import (
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

// ResolveSupervisorRecipients 解析工单主管收件人：维修组组长（LeaderUserID）
// 与持有 ticket.assign 权限的成员（服务经理/管理员）的并集。
// 依据用户决策：超时/升级告警一次全部通知，不逐级升级。
func ResolveSupervisorRecipients(ticket *models.Ticket) ([]NotificationRecipient, error) {
	if ticket == nil || ticket.TenantID <= 0 {
		return []NotificationRecipient{}, nil
	}
	recipients, err := NotificationAudienceService.ResolveTenantRecipients(
		ticket.TenantID,
		constants.PermissionTicketView.Code,
		constants.PermissionTicketAssign.Code,
	)
	if err != nil {
		return nil, err
	}
	team := resolveTicketRepairTeam(ticket)
	if team == nil || team.LeaderUserID <= 0 {
		return recipients, nil
	}
	seen := make(map[int64]bool, len(recipients)+1)
	for _, recipient := range recipients {
		seen[recipient.UserID] = true
	}
	if seen[team.LeaderUserID] {
		return recipients, nil
	}
	leaderMembers, err := NotificationAudienceService.ResolveTenantRecipients(ticket.TenantID)
	if err != nil {
		return recipients, nil
	}
	for _, member := range leaderMembers {
		if member.UserID == team.LeaderUserID {
			recipients = append(recipients, member)
			seen[member.UserID] = true
			break
		}
	}
	return recipients, nil
}

// ResolveSupervisorOwner 返回接单多次超时后的兜底负责人。对于产品维修组内工单，
// 兜底负责人必须是当前维修组成员，避免自动派给无法受理该组工单的人。
func ResolveSupervisorOwner(ticket *models.Ticket) int64 {
	if ticket == nil || ticket.TenantID <= 0 {
		return 0
	}
	team := resolveTicketRepairTeam(ticket)
	if team != nil && team.ID > 0 {
		if canSupervisorTakeOverTicket(ticket, team.ID, team.LeaderUserID) {
			return team.LeaderUserID
		}
		recipients, err := ResolveSupervisorRecipients(ticket)
		if err != nil {
			return 0
		}
		for _, recipient := range recipients {
			if canSupervisorTakeOverTicket(ticket, team.ID, recipient.UserID) {
				return recipient.UserID
			}
		}
		return 0
	}

	recipients, err := ResolveSupervisorRecipients(ticket)
	if err != nil {
		return 0
	}
	for _, recipient := range recipients {
		if activeSupervisorUser(ticket, recipient.UserID) {
			return recipient.UserID
		}
	}
	return 0
}

func canSupervisorTakeOverTicket(ticket *models.Ticket, teamID int64, userID int64) bool {
	return activeSupervisorUser(ticket, userID) &&
		AgentTeamMemberService.IsUserActiveMemberOfTeamDB(sqls.DB(), ticket.TenantID, teamID, userID) &&
		supervisorTakeoverAvailable(ticket.TenantID, userID)
}

func activeSupervisorUser(ticket *models.Ticket, userID int64) bool {
	if ticket == nil || userID <= 0 || userID == ticket.CurrentAssigneeID {
		return false
	}
	user := repositories.UserRepository.Get(sqls.DB(), userID)
	return user != nil && user.Status == enums.StatusOk
}

func supervisorTakeoverAvailable(tenantID, userID int64) bool {
	if tenantID <= 0 || userID <= 0 {
		return false
	}
	db := sqls.DB()
	if db == nil {
		return false
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("user_id", userID).
		Eq("status", enums.StatusOk))
	if profile == nil || profile.ServiceStatus != enums.ServiceStatusIdle {
		return false
	}
	if !AgentWorkStatusService.IsDispatchAvailable(tenantID, userID) {
		return false
	}
	return isDispatchReachable(profile, time.Now())
}

func resolveTicketRepairTeam(ticket *models.Ticket) *models.AgentTeam {
	if ticket == nil || ticket.TenantID <= 0 {
		return nil
	}
	if ticket.CurrentTeamID > 0 {
		teams := AgentTeamService.FindByIds([]int64{ticket.CurrentTeamID})
		if len(teams) > 0 && teams[0].TenantID == ticket.TenantID && teams[0].Status == enums.StatusOk &&
			(ticket.ProductID <= 0 || teams[0].ProductID <= 0 || teams[0].ProductID == ticket.ProductID) {
			return &teams[0]
		}
	}
	if ticket.ProductID > 0 {
		return ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), ticket.TenantID, ticket.ProductID)
	}
	return nil
}

// NotifySupervisors 向工单主管发送站内告警。idempotencyKeySuffix 用于幂等，
// 同一工单同一事件（如按小时限频的派单失败、每次接单超时）不重复通知。
func NotifySupervisors(ticket *models.Ticket, title, content, notificationType, idempotencyKeySuffix string) error {
	if ticket == nil || ticket.TenantID <= 0 {
		return nil
	}
	recipients, err := ResolveSupervisorRecipients(ticket)
	if err != nil {
		slog.Warn("resolve supervisor recipients failed", "ticket_id", ticket.ID, "error", err)
		return err
	}
	if len(recipients) == 0 {
		return nil
	}
	req := request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		Title:            strings.TrimSpace(title),
		Content:          strings.TrimSpace(content),
		NotificationType: notificationType,
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            "warning",
		Channels:         "in_app",
		IdempotencyKey:   "ticket.alert:" + idempotencyKeySuffix,
	}
	return NotificationAudienceService.Deliver(recipients, req)
}
