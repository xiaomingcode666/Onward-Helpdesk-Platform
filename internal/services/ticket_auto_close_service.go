package services

import (
	"log/slog"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var TicketAutoCloseService = newTicketAutoCloseService()

type ticketAutoCloseService struct {
	now func() time.Time
}

func newTicketAutoCloseService() *ticketAutoCloseService {
	return &ticketAutoCloseService{now: time.Now}
}

func (s *ticketAutoCloseService) GetPolicy(tenantID int64) (*dto.TicketAutoClosePolicyDTO, error) {
	tenant := repositories.TenantRepository.Get(sqls.DB(), tenantID)
	if tenant == nil {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	return &dto.TicketAutoClosePolicyDTO{Enabled: tenant.TicketAutoCloseEnabled, Days: normalizeAutoCloseDays(tenant.TicketAutoCloseDays)}, nil
}

func (s *ticketAutoCloseService) UpdatePolicy(tenantID int64, policy dto.TicketAutoClosePolicyDTO, operator *dto.AuthPrincipal) (*dto.TicketAutoClosePolicyDTO, error) {
	if operator == nil || operator.TenantID != tenantID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	days := normalizeAutoCloseDays(policy.Days)
	if err := repositories.TenantRepository.Updates(sqls.DB(), tenantID, map[string]any{
		"ticket_auto_close_enabled": policy.Enabled,
		"ticket_auto_close_days":    days,
		"updated_at":                s.now(),
		"update_user_id":            operator.UserID,
		"update_user_name":          operator.Username,
	}); err != nil {
		return nil, err
	}
	return &dto.TicketAutoClosePolicyDTO{Enabled: policy.Enabled, Days: days}, nil
}

func (s *ticketAutoCloseService) CloseDueTickets(limitPerTenant int) int {
	closed := 0
	runAt := s.now()
	for _, tenant := range repositories.TenantRepository.FindAutoCloseEnabled(sqls.DB()) {
		days := normalizeAutoCloseDays(tenant.TicketAutoCloseDays)
		cutoff := runAt.AddDate(0, 0, -days)
		for _, ticket := range repositories.TicketRepository.FindDueForAutoClose(sqls.DB(), tenant.ID, cutoff, limitPerTenant) {
			if !s.eligibleAt(ticket, runAt) {
				continue
			}
			operator := &dto.AuthPrincipal{
				Username:    "ticket-auto-close",
				Nickname:    "Ticket Auto Close",
				TenantID:    tenant.ID,
				DomainType:  "service_account",
				SubjectType: "service_account",
			}
			if err := TicketLifecycleService.Close(ticket.ID, "客户确认期已结束，系统自动关闭", operator); err != nil {
				slog.Warn("auto close ticket skipped", "ticket_id", ticket.ID, "tenant_id", tenant.ID, "error", err)
				continue
			}
			closed++
		}
	}
	return closed
}

func (s *ticketAutoCloseService) eligibleAt(ticket models.Ticket, runAt time.Time) bool {
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status != enums.TicketStatusResolved && status != enums.TicketStatusPendingCustomerConfirm {
		return false
	}
	if ticket.ResolvedAt == nil || ticket.ResolvedAt.After(runAt) {
		return false
	}
	if ticket.ProductModuleID > 0 {
		module := repositories.ProductModuleRepository.Get(sqls.DB(), ticket.ProductModuleID)
		if module != nil && module.IsSafetyCritical {
			return false
		}
	}
	if repositories.TicketSupplierCollaborationRepository.CountActiveByTicket(sqls.DB(), ticket.TenantID, ticket.ID) > 0 {
		return false
	}
	return !repositories.TicketRepository.HasUnfinishedMeeting(sqls.DB(), ticket.ID)
}

func normalizeAutoCloseDays(days int) int {
	if days <= 0 {
		return 7
	}
	if days > 90 {
		return 90
	}
	return days
}
