package services

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketKnowledgeCandidateService = newTicketKnowledgeCandidateService()

type ticketKnowledgeCandidateService struct{}

type TicketKnowledgeCandidateCreateResult struct {
	Candidate *models.KnowledgeCandidate
	Created   bool
}

func newTicketKnowledgeCandidateService() *ticketKnowledgeCandidateService {
	return &ticketKnowledgeCandidateService{}
}

func (s *ticketKnowledgeCandidateService) List(tenantID, ticketID int64) ([]models.KnowledgeCandidate, error) {
	if _, err := s.requireTicket(tenantID, ticketID); err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return []models.KnowledgeCandidate{}, nil
	}
	return repositories.KnowledgeCandidateRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("ticket_id", ticketID).
		NotEq("status", enums.StatusDeleted).
		Desc("created_at").
		Desc("id")), nil
}

func (s *ticketKnowledgeCandidateService) Create(tenantID, ticketID int64, req request.CreateTicketKnowledgeCandidateRequest, operator *dto.AuthPrincipal) (*TicketKnowledgeCandidateCreateResult, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket, err := s.requireTicket(tenantID, ticketID)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return nil, errorsx.Forbidden("tenant AI capability is disabled")
	}
	if existing := s.findActiveCandidate(tenantID, ticketID); existing != nil {
		return &TicketKnowledgeCandidateCreateResult{Candidate: existing}, nil
	}
	if req.KnowledgeBaseID > 0 {
		knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), req.KnowledgeBaseID)
		if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("knowledge base does not belong to this tenant")
		}
	}
	repairs := repositories.TicketRepairRepository.FindByTicketID(sqls.DB(), ticket.ID)
	rootCause := firstNonEmptyString(strings.TrimSpace(req.RootCauseSummary), TicketLifecycleService.buildRootCauseSummary(repairs))
	solution := firstNonEmptyString(strings.TrimSpace(req.SolutionSummary), TicketLifecycleService.buildSolutionSummary(repairs), ticket.DiagnosisSummary)
	suggestion := firstNonEmptyString(strings.TrimSpace(req.Suggestion), solution, ticket.SymptomSummary, ticket.Description)
	title := buildTicketKnowledgeCandidateTitle(strings.TrimSpace(req.Title), ticket, repairs)
	candidate := &models.KnowledgeCandidate{
		TenantID:         tenantID,
		ProductID:        ticket.ProductID,
		ProductModelID:   ticket.ProductModelID,
		SourceType:       "ticket_manual",
		SourceID:         ticket.TicketNo,
		TicketID:         ticket.ID,
		Title:            title,
		Suggestion:       suggestion,
		RootCauseSummary: rootCause,
		SolutionSummary:  solution,
		KnowledgeBaseID:  req.KnowledgeBaseID,
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	}
	created := false
	saved := candidate
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := KnowledgeCandidateScoringService.PrepareCandidateDB(tx, candidate, ticket, repairs); err != nil {
			return err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(candidate)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			existing := s.findActiveCandidateDB(tx, tenantID, ticketID)
			if existing == nil {
				return errorsx.InvalidParam("knowledge candidate could not be created")
			}
			saved = existing
			return nil
		}
		created = true
		if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, candidate.SimilarityHash); err != nil {
			return err
		}
		eventID := "tenant:" + strconv.FormatInt(candidate.TenantID, 10) + ":knowledge.candidate.created:" + strconv.FormatInt(candidate.ID, 10)
		_, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
			TenantID:       candidate.TenantID,
			IdempotencyKey: eventID,
			EventType:      events.EventKnowledgeCandidateCreated,
			Payload: events.KnowledgeCandidateCreatedEvent{
				EventID:     eventID,
				CandidateID: candidate.ID,
				SourceType:  candidate.SourceType,
				SourceID:    candidate.SourceID,
				TenantID:    candidate.TenantID,
				Suggestion:  candidate.Suggestion,
			},
			Source:      "ticket_knowledge_candidate_service",
			AggregateID: strconv.FormatInt(candidate.ID, 10),
			ActorID:     strconv.FormatInt(operator.UserID, 10),
			ActorType:   "user",
			CreatedAt:   candidate.CreatedAt,
		})
		return err
	}); err != nil {
		return nil, err
	}
	if !created && saved.ScoreVersion != enums.KnowledgeCandidateScoreVersion {
		if err := KnowledgeCandidateScoringService.RescoreCandidateDB(sqls.DB(), saved); err != nil {
			return nil, err
		}
	}
	if current := repositories.KnowledgeCandidateRepository.Get(sqls.DB(), saved.ID); current != nil {
		saved = current
	}
	if created {
		eventbus.WakeDefaultOutboxPublisher()
	}
	return &TicketKnowledgeCandidateCreateResult{Candidate: saved, Created: created}, nil
}

func (s *ticketKnowledgeCandidateService) findActiveCandidate(tenantID, ticketID int64) *models.KnowledgeCandidate {
	return s.findActiveCandidateDB(sqls.DB(), tenantID, ticketID)
}

func (s *ticketKnowledgeCandidateService) findActiveCandidateDB(db *gorm.DB, tenantID, ticketID int64) *models.KnowledgeCandidate {
	return repositories.KnowledgeCandidateRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("ticket_id", ticketID).
		NotEq("status", enums.StatusDeleted).
		Desc("knowledge_entry_id").
		Desc("id"))
}

func (s *ticketKnowledgeCandidateService) requireTicket(tenantID, ticketID int64) (*models.Ticket, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, errorsx.InvalidParam("tenant and ticket are required")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	return ticket, nil
}

func buildTicketKnowledgeCandidateTitle(requested string, ticket *models.Ticket, repairs []models.TicketRepairRecord) string {
	requested = compactKnowledgeCandidateTitle(requested)
	if requested != "" {
		return requested
	}
	faultCode := ""
	ticketTitle := ""
	if ticket != nil {
		faultCode = compactKnowledgeCandidateTitle(ticket.FaultCode)
		ticketTitle = compactKnowledgeCandidateTitle(ticket.Title)
	}
	detail := compactKnowledgeCandidateTitle(latestRepairKnowledgeDigest(repairs))
	if faultCode != "" {
		if detail != "" {
			return trimKnowledgeCandidateTitle(strings.TrimSpace(faultCode + " " + strings.TrimPrefix(detail, faultCode)))
		}
		if ticketTitle != "" && !strings.EqualFold(ticketTitle, faultCode) {
			return trimKnowledgeCandidateTitle(strings.TrimSpace(faultCode + " " + strings.TrimPrefix(ticketTitle, faultCode)))
		}
		return faultCode
	}
	if ticketTitle != "" {
		if detail != "" && !strings.Contains(ticketTitle, detail) {
			return trimKnowledgeCandidateTitle(ticketTitle + " " + detail)
		}
		return ticketTitle
	}
	if detail != "" {
		return detail
	}
	return "维修知识候选"
}

func latestRepairKnowledgeDigest(repairs []models.TicketRepairRecord) string {
	for i := len(repairs) - 1; i >= 0; i-- {
		repair := repairs[i]
		if digest := firstNonEmptyString(repair.RootCause, repair.Solution, repair.Conclusion); strings.TrimSpace(digest) != "" {
			return digest
		}
	}
	return ""
}

func compactKnowledgeCandidateTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", "；", "，", ";", "，", "。", "，")
	value = replacer.Replace(value)
	fields := strings.Fields(value)
	if len(fields) > 0 {
		value = strings.Join(fields, " ")
	}
	if idx := strings.Index(value, "，"); idx > 0 {
		value = value[:idx]
	}
	return trimKnowledgeCandidateTitle(strings.TrimSpace(value))
}

func trimKnowledgeCandidateTitle(value string) string {
	const maxRunes = 56
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}
