package services

import (
	"strings"
	"time"
	"unicode"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
)

type TicketDuplicateCandidate struct {
	TicketRelationView
	Revision  int64     `json:"revision"`
	Match     string    `json:"match"`
	CreatedAt time.Time `json:"created_at"`
}

func ticketCreatedBefore(parent, child models.Ticket) bool {
	return parent.CreatedAt.Before(child.CreatedAt) || (parent.CreatedAt.Equal(child.CreatedAt) && parent.ID < child.ID)
}

// Each ticket keeps its own clock. Audit snapshots are immutable, including when
// the relation is later removed. No source content or attachment is transferred.
func ticketRelationSLASnapshot(t models.Ticket, now time.Time) map[string]any {
	resolutionEnd := now
	if t.ResolvedAt != nil {
		resolutionEnd = *t.ResolvedAt
	} else if ticketCaseClosed(t) && t.HandledAt != nil {
		resolutionEnd = *t.HandledAt
	}
	acceptEnd := now
	if t.AcceptedAt != nil {
		acceptEnd = *t.AcceptedAt
	} else if ticketCaseClosed(t) && t.HandledAt != nil {
		acceptEnd = *t.HandledAt
	}
	return map[string]any{
		"ticket_id": t.ID, "ticket_no": t.TicketNo, "recorded_at": now,
		"created_at": t.CreatedAt, "assigned_at": t.AssignedAt, "accepted_at": t.AcceptedAt,
		"resolved_at": t.ResolvedAt, "handled_at": t.HandledAt,
		"status": models.EffectiveTicketCaseStatus(t), "priority": ticketEffectivePriority(t),
		"previous_sla_deadline": t.SLADueAt, "sla_deadline": t.SLADueAt,
		"previous_accept_deadline": t.AcceptDeadlineAt, "accept_deadline": t.AcceptDeadlineAt,
		"resolution_overdue": t.SLADueAt != nil && resolutionEnd.After(*t.SLADueAt),
		"accept_overdue":     t.AcceptDeadlineAt != nil && acceptEnd.After(*t.AcceptDeadlineAt),
	}
}

// A match is only a suggestion. Staff confirm that reports refer to the same
// occurrence. Older root tickets are presented first; tenants and visibility are
// checked independently of customer identity (one incident can affect many users).
func SearchTicketDuplicateCandidates(ticketID int64, query string, op *dto.AuthPrincipal) ([]TicketDuplicateCandidate, error) {
	if op == nil {
		return nil, errorsx.Forbidden("请登录后操作")
	}
	db := sqls.DB()
	var current models.Ticket
	if err := db.Where("tenant_id = ? AND id = ?", op.EffectiveTenantID(), ticketID).First(&current).Error; err != nil {
		return nil, err
	}
	if !canManageTicketGovernance(&current, op) {
		return nil, errorsx.Forbidden("需要工单管理权限")
	}
	result := []TicketDuplicateCandidate{}
	if current.MergedIntoID > 0 || current.Status == "draft" {
		return result, nil
	}
	var relations []models.TicketRelation
	if err := db.Where("tenant_id = ? AND kind = 'parent' AND active_key IS NOT NULL", current.TenantID).Find(&relations).Error; err != nil {
		return nil, err
	}
	children := map[int64]bool{}
	for _, r := range relations {
		if r.SourceID == current.ID || r.TargetID == current.ID {
			return result, nil
		}
		children[r.TargetID] = true
	}
	q := db.Where("tenant_id = ? AND merged_into_id = 0 AND (created_at < ? OR (created_at = ? AND id < ?))", current.TenantID, current.CreatedAt, current.CreatedAt, current.ID)
	query = strings.TrimSpace(query)
	if query != "" {
		q = q.Where("ticket_no LIKE ? OR title LIKE ?", "%"+query+"%", "%"+query+"%")
	}
	var rows []models.Ticket
	if err := q.Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, t := range rows {
		if children[t.ID] || ticketCaseClosed(t) || t.Status == "draft" || !canManageTicketGovernance(&t, op) {
			continue
		}
		match := "manual"
		if normalizedDuplicateTitle(current.Title) != "" && normalizedDuplicateTitle(current.Title) == normalizedDuplicateTitle(t.Title) {
			match = "same_title"
		} else if current.DeviceID > 0 && current.DeviceID == t.DeviceID {
			match = "same_device"
		}
		if query == "" && match == "manual" {
			continue
		}
		result = append(result, TicketDuplicateCandidate{TicketRelationView: TicketRelationView{OtherID: t.ID, TicketNo: t.TicketNo, Title: t.Title, Priority: ticketEffectivePriority(t), CaseType: t.CaseType, Status: models.EffectiveTicketCaseStatus(t), OwnerID: t.CaseOwnerID}, Revision: t.GovernanceRevision, Match: match, CreatedAt: t.CreatedAt})
		if len(result) == 100 {
			break
		}
	}
	return result, nil
}

func normalizedDuplicateTitle(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
